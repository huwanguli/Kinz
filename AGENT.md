# AGENT.md

This file provides guidance to AI coding agents (Claude Code, Copilot, Gemini CLI, etc.) working in this repository. It is tool-agnostic by design.

## Project Overview

Kinz is a lightweight TCP server framework written in Go (Go 1.25), being refactored from the legacy Zinx codebase into a production-ready framework. The refactor runs in phases P0–P6 (see `docs/superpowers/specs/2026-08-14-zinx-production-refactor-design.md`); the repository is currently at the **end of P2** (core implementation rewrite).

Module name: `kinz`. Packages: `kiface` (contract layer), `knet` (implementations), `klog` (slog-based logger), `kconf` (YAML config), `kpool` (sync.Pool buffers). Planned in later phases: `kmetrics`, `kmcp` (MCP server), `configs/`, `cmd/`.

## Build & Test Commands

```bash
# Build
go build ./...

# Vet
go vet ./...

# Tests
go test ./...                     # all packages
go test ./knet/ -v                # runtime + integration tests (real TCP)
go test ./kinterceptor/ -v        # frame decoder + chain tests
go test ./klog/ -v                # slog logger tests
go test ./kconf/ -v               # config loading chain tests
go test ./kpool/ -v               # buffer pool tests

# Coverage
go test -cover ./...
go test ./knet/ -coverprofile=coverage.out
go tool cover -func=coverage.out

# Race detector
go test -race ./...
```

> Note: `go test -race` requires CGO + a C toolchain (gcc/ld). It does **not** run on this dev machine (no compiler installed); use CI (ubuntu) or install mingw. All other commands run locally.

## Current State (end of P2)

- **Server** (`knet/server.go`): `Run(ctx)` / `Shutdown(ctx)` / `Serve(ctx)` lifecycle with graceful shutdown (stop accepting → drain connections → stop worker pool), `Address()` for ephemeral ports, max-conn rejection (sends `kiface.ServerFullMsgID` then closes), heartbeat template wiring, option-based construction (`WithConfig`/`WithMaxConn`/`WithName`).
- **Connection** (`knet/connection.go`): reader/writer goroutines, buffered write queue with timeout, atomic liveness tracking (`IsAlive`/`touch`), idempotent `Stop` via `sync.Once`, pooled read buffer (`kpool`), per-connection decoder clone.
- **Routing** (`knet/msgHandler.go` + `routerSlices.go`): classic `IRouter` and function-style `IRouterSlices` with global `Use` / range `Group` middleware, `Abort`, panic recovery per message, worker pool with graceful drain (`StopWorkerPool`), blocking backpressure. Middleware (`Use`/`Group`) is the single pipeline mechanism — the old interceptor chain was removed as redundant (it is fully covered by middleware, which can replace messages via `req.SetMessage`).
- **Heartbeat** (`knet/heartbeat.go`): interval + timeout (default 3×interval), any received message refreshes liveness, `OnRemoteNotAlive` defaults to graceful close, clone-per-connection.
- **Codec** (`knet/datapack.go`): `knet.TLVPack` implements the single `kiface.ICodec` seam (framing + TLV parse + Pack in one unit, `Clone` per connection). Handles sticky/half packets internally, configurable byte order, returns `ErrTooLargePacket` on oversize; decoded payloads are copied so asynchronous processing is safe. Custom wire formats implement one `ICodec` (no separate frame decoder / packet pair).
- **Config** (`kconf`): defaults → `conf/kinz.yaml` (missing file is fine) → `KINZ_*` env vars; durations accept "10s" strings or nanosecond ints.
- **kpool**: 4K/16K/64K size classes backed by `sync.Pool`.
- **Client** (`knet/client.go`) is still a stub (full implementation lands in P3).

## Code Conventions

- **Interface-first**: define the contract in `kiface` before implementing in `knet`.
- **Convention-first**: default paths are production-safe (heartbeat, max-conn rejection, panic recovery, graceful shutdown); extension happens at seams (`ICodec`, `RouterHandler` middleware via `Use`/`Group`, `IRouter`/`IRouterSlices`, `ILogger`, `IMetrics`).
- **Middleware contract**: a function-style handler must call `req.RouterSlicesNext()` to continue the chain (gin-style); `req.Abort()` stops it.
- **Errors**: use sentinel errors from `kiface`, wrap with `%w`. No panics in library code paths.
- **Byte order**: a wire-protocol decision — always explicit `binary.ByteOrder`, configurable (see `DataPack`/`NewDataPackWithOrder`); never probe host endianness.
- **Logging**: use `klog` (slog). No `fmt.Printf` in framework code.
- **Tests**: every feature ships its tests in the same commit; coverage gates per phase (P2: kconf ≥ 80%, kpool ≥ 80%, kinterceptor ≥ 70%, knet ≥ 55%).
- **Naming**: framework brand Kinz; packages `kin*`; exported symbols get English doc comments.

## Directory Layout

```
kiface/        contracts (interfaces, sentinel errors)
knet/          runtime (Server, Connection, MsgHandler, ConnManager,
               HeartBeatChecker, Client, DataPack, Message, Request)
klog/          ILogger + slog default implementation
kconf/         Config (defaults / YAML / env)
kpool/         size-classed sync.Pool buffers
examples/      runnable demos (ping)
docs/          design spec + implementation plans + interview guide
.github/       CI reference workflow (not required to run locally)
```

Planned in later phases: `kmetrics`, `kmcp` (MCP server), `configs/` (kinz.yaml sample), `cmd/`, `kinzctl` codegen (post-release P7).

## Common Pitfalls

- `go tool cover -func=coverage.out` needs an absolute path on this Windows setup; a relative `-coverprofile=cover.out` may not be written as expected.
- Constructing `kconf.Config` with a struct literal zeroes default fields (e.g. `MaxPacketSize`) — build configs from `kconf.Default()` and override, as the integration tests do.
- Files were migrated from legacy names: any `zinx`/`ziface`/`znet` reference in new code is a bug.
- The `ICodec` seam replaced the old `IDecoder`+`IDataPack` pair — custom protocols implement one `ICodec`.
- The interceptor chain (`IInterceptor`/`Chain`) was removed — middleware (`Use`/`Group`) covers all its cases (including message replacement via `req.SetMessage`). Do not reintroduce a second pipeline mechanism.
- `knet` integration tests bind `127.0.0.1:0` (ephemeral ports); they take ~6s due to the heartbeat-timeout case.
