# AGENT.md

This file provides guidance to AI coding agents (Claude Code, Copilot, Gemini CLI, etc.) working in this repository. It is tool-agnostic by design.

## Project Overview

Kinz is a lightweight TCP server framework written in Go (Go 1.25), being refactored from the legacy Zinx codebase into a production-ready framework. The refactor runs in phases P0–P6 (see `docs/superpowers/specs/2026-08-14-zinx-production-refactor-design.md`); the repository is currently at the **end of P1** (rename + contract layer).

Module name: `kinz`. Packages: `kiface` (contract layer), `knet` (implementations), `klog` (slog-based logger), `kinterceptor` (frame decoder + interceptor chain). Planned in later phases: `kconf` (YAML config), `kmetrics`, `kmcp` (MCP server), `kpool` (sync.Pool buffers), `examples/`, `configs/`, `cmd/`.

## Build & Test Commands

```bash
# Build
go build ./...

# Vet
go vet ./...

# Tests
go test ./...            # all packages
go test ./knet/ -v       # TLV codec tests
go test ./klog/ -v       # slog logger tests

# Coverage
go test -cover ./...
go test ./knet/ -coverprofile=coverage.out
go tool cover -func=coverage.out

# Race detector
go test -race ./...
```

> Note: `go test -race` requires CGO + a C toolchain (gcc/ld). It does **not** run on this dev machine (no compiler installed); use CI (ubuntu) or install mingw. All other commands run locally.

## Current State (end of P1)

- Legacy `demo/`, `mmo_game_zinx/`, `utils/` were archived (deleted; recoverable via git history). The protobuf dependency was dropped.
- `kiface` defines the full contract layer with English doc comments and sentinel errors (`ErrServerClosed`, `ErrConnClosed`, `ErrTooLargePacket`, `ErrServerFull`, `ErrProtocol`, `ErrTimeout`, `ErrConnNotFound`, `ErrMsgIDRegistered`).
- Compile-time interface assertions live in `knet/interface_test.go` and `kinterceptor/interface_test.go` — a signature drift fails the build.
- `knet` is mostly stubs (`Server`/`Connection`/`MsgHandler`/`HeartBeatChecker`/`Client` return `ErrNotImplemented`); `DataPack`/`Message` are real and tested (TLV, configurable byte order). `ConnManager` already enforces the max-connection limit.
- `klog` is a real `log/slog`-based logger (levels, JSON handler, dynamic level, `With` fields, legacy `InfoF`/`ErrorF`).
- `kinterceptor/framedecoder.go` still uses panics for protocol errors; the conversion to error returns lands in P2.

## Code Conventions

- **Interface-first**: define the contract in `kiface` before implementing in `knet`.
- **Convention-first**: default paths are production-safe (heartbeat, max-conn rejection, panic recovery, graceful shutdown); extension happens at seams (`IDecoder`, `IInterceptor`, `IRouter`, `ILogger`, `IMetrics`).
- **Errors**: use sentinel errors from `kiface`, wrap with `%w`. No panics in library code paths (the FrameDecoder panic conversion is scheduled P2 work).
- **Byte order**: a wire-protocol decision — always explicit `binary.ByteOrder`, configurable (see `DataPack`/`NewDataPackWithOrder`); never probe host endianness.
- **Logging**: use `klog` (slog). No `fmt.Printf` in framework code.
- **Tests**: every feature ships its tests in the same commit (each phase has a coverage gate; codec layer ≥ 80%).
- **Naming**: framework brand Kinz; packages `kin*`; exported symbols get English doc comments.

## Directory Layout

```
kiface/        contracts (interfaces, sentinel errors)
knet/          implementations (Server, Connection, MsgHandler, ConnManager,
               HeartBeatChecker, Client, DataPack, Message)
klog/          ILogger + slog default implementation
kinterceptor/  FrameDecoder (LengthField) + Chain
docs/          design spec + implementation plans + interview guide
.github/       CI reference workflow (not required to run locally)
```

## Common Pitfalls

- `go tool cover -func=coverage.out` needs an absolute path on this Windows setup; a relative `-coverprofile=cover.out` may not be written as expected.
- Files were migrated from legacy names: any `zinx`/`ziface`/`znet` reference in new code is a bug.
- `knet` package coverage is diluted by stubs (no logic) — judge coverage on real code (codec, klog), not package totals, until P2 lands.
