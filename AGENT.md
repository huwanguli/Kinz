# AGENT.md

This file provides guidance to AI coding agents (Claude Code, Copilot, Gemini CLI, etc.) working in this repository. It is tool-agnostic by design.

## Project Overview

Kinz is a lightweight TCP server framework written in Go (Go 1.25), refactored from the legacy Zinx codebase into a production-ready framework. The refactor ran in phases P0–P6 (see `docs/superpowers/specs/2026-08-14-zinx-production-refactor-design.md`); the repository is at **v1.0.0 (end of P6)**, tagged and released. Post-release P7 (`kinzctl` codegen) remains a documented option, not started.

Module name: `kinz`. Packages: `kiface` (contracts), `knet` (runtime), `klog` (slog logger), `kconf` (YAML config), `kpool` (sync.Pool buffers), `kmetrics` (prometheus/client_golang adapter), `kmcp` (optional MCP server on mark3labs/mcp-go).

## Build & Test Commands

```bash
# Build
go build ./...

# Vet
go vet ./...

# Tests
go test ./...            # all packages
go test ./knet/ -v       # runtime + integration tests (real TCP)
go test ./kmetrics/ -v   # metrics adapter tests
go test ./kmcp/ -v       # MCP tests (mcp-go client driven)
go test ./klog/ -v       # slog logger + ring buffer tests
go test ./kconf/ -v      # config loading chain tests
go test ./kpool/ -v      # buffer pool tests

# Coverage
go test -cover ./...
go test ./knet/ -coverprofile=coverage.out
go tool cover -func=coverage.out

# Race detector
go test -race ./...
```

> Note: `go test -race` requires CGO + a C toolchain (gcc/ld). It does **not** run on this dev machine (no compiler installed); use CI (ubuntu) or install mingw. All other commands run locally.

## Current State (v1.0.0, end of P6)

- **Server** (`knet/server.go`): `Run(ctx)` / `Shutdown(ctx)` / `Serve(ctx)` lifecycle with graceful shutdown, `Address()`, max-conn rejection (sends `ServerFullMsgID` then closes), heartbeat template wiring, options (`WithConfig`/`WithMaxConn`/`WithName`/`WithTLS`), Prometheus `/metrics` via `AttachMetrics(addr)`, `GetMetrics()`.
- **Connection** (`knet/connection.go`): reader/writer goroutines, buffered write queue with timeout, atomic liveness, idempotent `Stop` via `sync.Once`, pooled read buffer (`kpool`), per-connection codec clone, metrics counters. `GetConn()` returns `net.Conn` (plain TCP or TLS).
- **Client** (`knet/client.go`): dial (TCP/TLS), auto-reconnect with exponential backoff + jitter (`WithReconnect`), heartbeat, routing, worker pool. Shares the connection lifecycle with Server via the internal `connHost` abstraction.
- **Routing** (`knet/msgHandler.go` + `routerSlices.go`): single function-style routing (`AddRouterSlices` + `Use`/`Group` middleware), onion-model before/after semantics, `Abort`, panic recovery per message, worker pool with graceful drain, blocking backpressure + `queue_full` metric.
- **Codec** (`knet/datapack.go`): `knet.TLVPack` implements the single `kiface.ICodec` seam (framing + TLV parse + Pack + `Clone` per connection). Default little-endian; `NewTLVPackWithOrder` for big-endian. Decoded payloads are copied so async processing is safe. Custom wire formats implement one `ICodec`.
- **Heartbeat** (`knet/heartbeat.go`): interval + timeout (default 3×interval), any-message liveness, `OnRemoteNotAlive` defaults to graceful close, clone-per-connection, `heartbeat_missed` metric.
- **Metrics** (`kmetrics`): adapter over `prometheus/client_golang` (write-only Counter/Gauge/Histogram handles, deduplicated by name). Reads via `Snapshot()` (MCP get_metrics) or `promhttp.Handler()` / `AttachMetrics`.
- **MCP** (`kmcp`): **opt-in** adapter on mark3labs/mcp-go, 10 tools + 4 resources, stdio + streamable HTTP transports (`ServeStdio` / `ServeHTTP` / `Handler`), `WithAuth` hook. Never imported by knet (core stays SDK-free).
- **TLS**: `WithTLS` (server) / `WithTLSClient` (client), self-signed-cert integration test.
- **Config** (`kconf`): defaults → `conf/kinz.yaml` → full `KINZ_*` env coverage.
- **klog**: slog-based logger + ring-buffer backend (`klog.NewRingBuffer`, `Lines(n)` for MCP get_logs).
- **kpool**: 4K/16K/64K size classes backed by `sync.Pool`.

## Docs Map

- `README.md` — GitHub 主页入口（特性、快速开始、性能、质量）
- `docs/architecture.md` — 包结构、数据流、生命周期、约定与扩展点
- `docs/protocol.md` — TLV 线协议、字节序、粘包/半包、自定义 ICodec
- `docs/configuration.md` — kconf 字段、YAML、env、Option
- `docs/getting-started.md` — 快速开始（服务端/客户端/中间件）
- `docs/testing.md` — 测试命令、分布、覆盖率门禁
- `docs/performance.md` — 基准测试基线（微基准/端到端吞吐/瓶颈分析，v1.0.0 实测数值）
- `docs/mcp.md` — MCP 接入、工具/资源清单
- `docs/faq.md` — 常见问题
- `docs/production-checklist.md` — 部署、调参、监控、安全
- `docs/INTERVIEW_GUIDE.md` — 面试指南（P6 同步更新）

## Code Conventions

- **Interface-first**: define the contract in `kiface` before implementing in `knet`.
- **Convention-first**: default paths are production-safe (heartbeat, max-conn rejection, panic recovery, graceful shutdown); extension happens at seams (`ICodec`, `RouterHandler` middleware via `Use`/`Group`, `ILogger`, `kmetrics.Registry`).
- **Middleware contract**: a handler must call `req.RouterSlicesNext()` to continue the chain (gin-style); `req.Abort()` stops the remaining handlers. The chain is **synchronous nesting (onion model)**: code AFTER `req.RouterSlicesNext()` runs once downstream handlers finish. `Abort()` still unwinds the stack, so upstream after-logic runs.
- **Errors**: use sentinel errors from `kiface`, wrap with `%w`. No panics in library code paths.
- **Byte order**: a wire-protocol decision — always explicit `binary.ByteOrder`, configurable (`NewTLVPackWithOrder`); never probe host endianness.
- **Logging**: use `klog` (slog). No `fmt.Printf` in framework code.
- **Tests**: every feature ships its tests in the same commit. Current coverage: kmetrics 89.6% / klog 96.3% / knet 74.0% / kconf 86.2% / kpool 100% / kmcp 82.4% (P6 gate: core ≥ 70%, all met). Fuzz targets: `FuzzTLVPackDecode` / `FuzzRingBuffer` / `FuzzLoadYAML`. Benchmark baselines live in `docs/performance.md` (regression reference).
- **Naming**: framework brand Kinz; packages `kin*`; exported symbols get English doc comments.

## Directory Layout

```
kiface/        contracts (interfaces, sentinel errors)
knet/          runtime (Server, Client, Connection, MsgHandler, ConnManager,
               HeartBeatChecker, TLVPack, Message, Request)
klog/          ILogger + slog default implementation + ring buffer
kconf/         Config (defaults / YAML / env)
kpool/         size-classed sync.Pool buffers
kmetrics/      measurement layer over prometheus/client_golang
kmcp/          optional MCP server (mark3labs/mcp-go, stdio/streamable HTTP)
examples/      runnable demos (echo, chatroom, auth-middleware, mcp-stdio)
docs/          architecture / protocol / configuration / getting-started / testing /
               performance / faq / mcp / production-checklist / INTERVIEW_GUIDE /
               superpowers specs+plans
.github/       CI reference workflow (not required to run locally)
```

Planned in later phases: `configs/` (kinz.yaml sample), `cmd/`, `kinzctl` codegen (post-release P7).

## Common Pitfalls

- `go tool cover -func=coverage.out` needs an absolute path on this Windows setup; a relative `-coverprofile=cover.out` may not be written as expected.
- Constructing `kconf.Config` with a struct literal zeroes default fields (e.g. `MaxPacketSize`) — build configs from `kconf.Default()` and override, as the integration tests do.
- Files were migrated from legacy names: any `zinx`/`ziface`/`znet`/`zinterceptor` reference in new code is a bug.
- The `ICodec` seam replaced the old `IDecoder`+`IDataPack` pair — custom protocols implement one `ICodec`; `DataPack` was renamed to `TLVPack`.
- The interceptor chain (`IInterceptor`/`Chain`) and the classic router (`IRouter`/`BaseRouter`) were removed — middleware (`Use`/`Group`) covers all their cases. Do not reintroduce a second pipeline mechanism.
- `knet` integration tests bind `127.0.0.1:0` (ephemeral ports); they take ~6s due to the heartbeat-timeout case.
- PowerShell mangles unquoted `-bench=.` (the `.` is parsed as an alias) — always quote: `-bench "."`. On Windows, `go tool cover -func` also wants an absolute profile path (see above).
- `kmcp.ServeHTTP(addr)` mounts the MCP endpoint at `/mcp` (mcp-go default); the bare `Handler()` answers on any path you mount it on. Tests use `http://host:port/mcp` for the real listener.
