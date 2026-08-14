# AGENT.md

This file provides guidance to AI coding agents (Claude Code, Copilot, Gemini CLI, etc.) working in this repository. It is tool-agnostic by design.

## Project Overview

Kinz is a lightweight TCP server framework written in Go (Go 1.25), being refactored from the legacy Zinx codebase into a production-ready framework. The refactor runs in phases P0–P6 (see `docs/superpowers/specs/2026-08-14-zinx-production-refactor-design.md`); the repository is currently at the **end of P2** (core implementation rewrite).

Module name: `kinz`. Packages: `kiface` (contract layer), `knet` (implementations), `klog` (slog-based logger), `kconf` (YAML config), `kpool` (sync.Pool buffers), `kmetrics` (zero-dep measurement layer + opt-in official-client bridge), `kmcp` (MCP server exposing the runtime to AI tools). Planned in later phases: `configs/`, `cmd/`.

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

## Current State (end of P4)

- **Server** (`knet/server.go`): `Run(ctx)` / `Shutdown(ctx)` / `Serve(ctx)` lifecycle with graceful shutdown, `Address()`, max-conn rejection, heartbeat template wiring, option-based construction (`WithConfig`/`WithMaxConn`/`WithName`/`WithTLS`), Prometheus `/metrics` endpoint via `AttachMetrics(addr)`, `GetMetrics()`.
- **Connection** (`knet/connection.go`): reader/writer goroutines, buffered write queue with timeout, atomic liveness, idempotent `Stop` via `sync.Once`, pooled read buffer, per-connection codec clone, metrics counters. `GetConn()` returns `net.Conn` (plain TCP or TLS).
- **Client** (`knet/client.go`): full implementation — dial (TCP or TLS), auto-reconnect with exponential backoff + jitter (`WithReconnect`), heartbeat, routing, worker pool. Shares the connection lifecycle with Server via the internal `connHost` abstraction.
- **Routing** (`knet/msgHandler.go` + `routerSlices.go`): single function-style routing (`AddRouterSlices` + `Use`/`Group` middleware) with onion-model before/after semantics, `Abort`, panic recovery, worker pool with graceful drain, blocking backpressure + `queue_full` metric.
- **Heartbeat** (`knet/heartbeat.go`): interval + timeout, any-message liveness, `OnRemoteNotAlive` defaults to graceful close, clone-per-connection, `heartbeat_missed` metric.
- **Metrics** (`kmetrics`): measurement layer built on the standard `prometheus/client_golang` (write-only Counter/Gauge/Histogram handles with kinz_* names, deduplicated by name). Reads go through `Snapshot()` (for MCP get_metrics) or the `promhttp.Handler()` / `Server.AttachMetrics(addr)` endpoint. No hand-written exposition format to maintain.
- **MCP** (`kmcp`): **opt-in** adapter built on mark3labs/mcp-go with 10 tools (server_info, list/get/send/broadcast/close connections, get_metrics, get_config, get_logs, shutdown_server) and 4 resources (connections://, metrics://, config://, logs://). Transports: stdio + streamable HTTP (`ServeStdio` / `ServeHTTP` / `Handler`), `WithAuth` hook. It is a standalone adapter (never imported by knet — the core stays SDK-free); wire it yourself: `kmcp.NewServer(srv, ...).ServeHTTP(addr)`.
- **TLS**: `WithTLS` (server) / `WithTLSClient` (client) with a self-signed-cert integration test.
- **Config** (`kconf`): defaults → `conf/kinz.yaml` → full `KINZ_*` env coverage.
- **klog**: slog-based logger + ring-buffer backend (`klog.NewRingBuffer`, `Lines(n)` for MCP get_logs).
- **kpool**: 4K/16K/64K size classes backed by `sync.Pool`.

## Code Conventions

- **Interface-first**: define the contract in `kiface` before implementing in `knet`.
- **Convention-first**: default paths are production-safe (heartbeat, max-conn rejection, panic recovery, graceful shutdown); extension happens at seams (`ICodec`, `RouterHandler` middleware via `Use`/`Group`, `ILogger`, `kmetrics.Registry`).
- **Middleware contract**: a handler must call `req.RouterSlicesNext()` to continue the chain (gin-style); `req.Abort()` stops the remaining handlers. The chain is **synchronous nesting (onion model)**: code written AFTER `req.RouterSlicesNext()` runs once the downstream handlers finish — use it for after-middleware (timing, recovery, response observability). `Abort()` still unwinds the stack, so upstream after-logic runs.
- **Errors**: use sentinel errors from `kiface`, wrap with `%w`. No panics in library code paths.
- **Byte order**: a wire-protocol decision — always explicit `binary.ByteOrder`, configurable (see `DataPack`/`NewDataPackWithOrder`); never probe host endianness.
- **Logging**: use `klog` (slog). No `fmt.Printf` in framework code.
- **Tests**: every feature ships its tests in the same commit; coverage gates per phase (P2: kconf ≥ 80%, kpool ≥ 80%, kinterceptor ≥ 70%, knet ≥ 55%).
- **Naming**: framework brand Kinz; packages `kin*`; exported symbols get English doc comments.

## Directory Layout

```
kiface/        contracts (interfaces, sentinel errors)
knet/          runtime (Server, Client, Connection, MsgHandler, ConnManager,
               HeartBeatChecker, DataPack/TLVPack, Message, Request)
klog/          ILogger + slog default implementation + ring buffer
kconf/         Config (defaults / YAML / env)
kpool/         size-classed sync.Pool buffers
kmetrics/      measurement layer over prometheus/client_golang (Snapshot + promhttp handler)
kmcp/          MCP server (mark3labs/mcp-go, stdio/streamable HTTP) exposing the runtime
examples/      runnable demos (echo server + client)
docs/          design spec + implementation plans + mcp.md + interview guide
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
