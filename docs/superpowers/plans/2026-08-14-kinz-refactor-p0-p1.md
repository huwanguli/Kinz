# Kinz 框架重构实施计划（P0–P1）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成 Kinz 重构的 Phase 0（基线锁定）与 Phase 1（改名 + 接口重定义），使 `go build ./...` 恢复绿色、测试可跑、CI 骨架落地。

**Architecture:** 在"接口层（kiface）— 实现层（knet）"骨架下：删除归档 demo/mmo/utils → module 改名 `kinz`、包改名 `kin*` → 按"约定为主 + 明确扩展点"哲学重写 kiface 接口 → 迁移保留的编解码层（TLV DataPack/Message，带真实单元测试）→ 其余实现以桩代码满足接口（P2 重写）。测试与功能同批交付。

**Tech Stack:** Go 1.25、`gopkg.in/yaml.v3`（P2 引入）、GitHub Actions、Makefile、标准库 testing/`-race`/`-cover`。

---

## 执行与评审协议（重要）

- **每个 Task = 一个评审单元**。Task 内的小步骤（写测试→跑→实现→再跑→提交）连续执行，不单独暂停。
- 每个 Task 完成后**暂停**，向用户汇报：改动文件清单、构建/测试/覆盖率输出、与本设计文档 §2 哲学的对应。**用户 review 通过后**才进入下一 Task。
- **覆盖率要求**：
  - **CI 不实际运行**：验证以本地命令（`go build` / `go vet` / `go test -race -cover`）为准；`.github/workflows/ci.yml` 仅作参考文件，不要求推送触发或保持运行。
- **目录结构（Go 库惯例，根级包，非 pkg/ 布局）**：`pkg/` 在 golang-standards 官方 README 中明确为"非 Go 惯例"；库采用根级包（同 gin/echo/原版 zinx）。`INTERVIEW_GUIDE.md` 移入 `docs/`，`.idea/` 解除 git 跟踪，`examples/`/`configs/`/`cmd/` 在对应阶段创建。
- 每个 Task 的新功能与其测试同批提交；
  - P1 目标：编解码层（`knet/message.go` + `knet/datapack.go`）行覆盖率 **≥ 80%**（`go tool cover -func` 验证）；桩代码不设覆盖率（无逻辑，P2 起核心包按阶段要求 ≥ 70%，P6 最终门禁）；
  - 每个 Task 的验证步骤都包含 `go test` 输出，覆盖率里程碑记录在 Task 5。
- 预期构建状态已标注在每个 Task 末尾（Task 0–2 预期 broken，Task 3 klog 可独立编译通过，Task 4 首次全绿），避免 review 时误判。

## 文件结构

| 文件 | 责任 | 动作 |
|------|------|------|
| `.github/workflows/ci.yml` | CI：build/vet/test(-race,-cover) | 新建（P0） |
| `Makefile` | 本地常用命令 | 新建（P0） |
| `docs/superpowers/plans/2026-08-14-baseline-build-errors.txt` | 基线构建失败清单 | 新建（P0） |
| `go.mod` | module `kinz`，移除 protobuf 依赖 | 修改（P1） |
| `kiface/errors.go` | 哨兵错误 | 新建（P1） |
| `kiface/{iserver,iconnection,imessage,irequest,imsgHandler,iconnmanager,iclient,idatapack,idecoder,ilengthfield,iinterceptor,iheartbeat,irouter}.go` | 接口契约层（英文注释） | 重写（P1） |
| `knet/message.go` | `Message` 实现新 `IMessage` | 重写（P1） |
| `knet/datapack.go` | TLV 编解码，去 utils 依赖、哨兵错误 | 重写（P1） |
| `knet/options.go` | `ClientOption` | 保留（仅改名） |
| `knet/{server,connection,msgHandler,connmanager,heartbeat,client}.go` | 桩实现（P2 重写） | 重写（P1） |
| `knet/datapack_test.go` | TLV 单元测试（替换旧手工脚本） | 重写（P1） |
| `knet/interface_test.go` | 编译期接口断言 | 新建（P1） |
| `kinterceptor/{chain,framedecoder,interceptor}.go` | 保留（改名 + 补 `GetLengthField`；错误化改造在 P2） | 改名（P1） |
| `kinterceptor/interface_test.go` | 编译期接口断言 | 新建（P1） |
| `klog/ilog.go`、`klog/log.go` | `ILogger` 接口 + `log/slog` 实现（级别/格式/输出可配，无硬编码颜色） | 重写（P1，用户要求提前） |
| `klog/log_test.go` | slog 级别过滤/JSON/With/InfoF 兼容测试 | 新建（P1） |
| `CLAUDE.md` | 构建命令同步（完整重写在 P5） | 小改（P1 Task 5） |

---

## Task 0: 基线锁定（P0）

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `Makefile`
- Create: `docs/superpowers/plans/2026-08-14-baseline-build-errors.txt`

- [ ] **Step 1: 创建 CI 骨架**

创建 `.github/workflows/ci.yml`：

```yaml
name: CI

on:
  push:
    branches: [master]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
      - name: Build
        run: go build ./...
      - name: Vet
        run: go vet ./...
      - name: Test (race + cover)
        run: go test -race -cover ./...
```

> 注：P0 阶段 CI 预期红（当前构建失败），P1 Task 3 后转绿。这是设计文档 §6.15 的"允许标红"策略。

- [ ] **Step 2: 创建 Makefile**

创建 `Makefile`：

```makefile
.PHONY: build vet test test-race cover

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

cover:
	go test -cover ./...
```

- [ ] **Step 3: 记录基线构建失败清单**

Run（工作目录：仓库根）：

```powershell
go build ./... 2>&1 | Out-File -Encoding utf8 docs/superpowers/plans/2026-08-14-baseline-build-errors.txt
```

Expected: 命令退出码非 0；文件内容与设计文档 §3.1 列出的错误一致（IDataPackd 拼写、IMsgHandle/DoMsgHandler、Request/Message/Connection/ConnManager/Client 缺失方法等）。

- [ ] **Step 4: 提交**

```bash
git add .github/workflows/ci.yml Makefile docs/superpowers/plans/2026-08-14-baseline-build-errors.txt
git commit -m "chore: add CI skeleton and record baseline build failures"
```

**Task 0 结束时的构建状态：预期 broken（基线）。**

---

## Task 1: 删除归档 + 模块与包改名

**Files:**
- Delete: `demo/`、`mmo_game_zinx/`、`utils/`（git rm）
- Delete: `knet/datapack_test.go`（旧手工脚本，Task 3 重建真测试）、`knet/{server,connection,msgHandler,connmanager,heartbeat,client,request}.go`（Task 4 重建桩）
- Modify: `go.mod`

- [ ] **Step 1: 删除归档与待重建文件**

```bash
git rm -r demo mmo_game_zinx utils
git rm znet/datapack_test.go znet/server.go znet/connection.go znet/msgHandler.go znet/connmanager.go znet/heartbeat.go znet/client.go znet/request.go
```

- [ ] **Step 2: 目录结构整理**

```bash
git mv INTERVIEW_GUIDE.md docs/INTERVIEW_GUIDE.md
git rm -r --cached .idea
```

更新 `.gitignore` 为：

```
# IDE
.idea/

# Go build/test artifacts
*.out
*.test
bin/

# legacy (deleted) references
myDemo/
```

- [ ] **Step 3: 模块改名 + 清理依赖**

修改 `go.mod` 为：

```
module kinz

go 1.25
```

Run：

```powershell
go mod tidy
```

Expected: `go.mod` 如上；`go.sum` 中的 protobuf 条目被清除（或文件变空/消失）。

- [ ] **Step 4: 包目录改名**

```bash
git mv ziface kiface
git mv znet knet
git mv zlog klog
git mv zinterceptor kinterceptor
```

- [ ] **Step 5: 批量替换包声明与导入路径**

Run（PowerShell，仓库根；`kinz/utils` 会在 Task 3 随 datapack 重写一并消失）：

```powershell
$map = [ordered]@{
  'package ziface'       = 'package kiface'
  'package znet'         = 'package knet'
  'package zlog'         = 'package klog'
  'package zinterceptor' = 'package kinterceptor'
  '"zinx/ziface"'        = '"kinz/kiface"'
  '"zinx/znet"'          = '"kinz/knet"'
  '"zinx/zlog"'          = '"kinz/klog"'
  '"zinx/zinterceptor"'  = '"kinz/kinterceptor"'
}
Get-ChildItem -Recurse -Filter *.go | ForEach-Object {
  $c = Get-Content $_.FullName -Raw
  foreach ($k in $map.Keys) { $c = $c.Replace($k, $map[$k]) }
  Set-Content -Path $_.FullName -Value $c -NoNewline -Encoding utf8
}
```

Verify：

```powershell
Get-ChildItem -Recurse -Filter *.go | Select-String -Pattern 'package zin|"zinx/' | Select-Object -First 5
```

Expected: 无输出（无残留 `zin` 字样）。注意 `kinterceptor/`、`klog/` 此步后即为最终形态（P2/P3 再改行为）。

- [ ] **Step 6: 提交**

```bash
git add kiface knet klog kinterceptor go.mod go.sum .gitignore docs/INTERVIEW_GUIDE.md
git commit -m "chore: rename framework to kinz, restructure repo, remove demo/mmo/utils"
```

**Task 1 结束时的构建状态：预期 broken（kiface 仍为旧接口内容；knet/datapack.go 仍引用已删除的 utils 与旧方法名）。**

---

## Task 2: kiface 接口层重写

**Files（全部重写，英文 doc comment）：**
- Create/Overwrite: `kiface/errors.go`、`kiface/iserver.go`、`kiface/iconnection.go`、`kiface/imessage.go`、`kiface/irequest.go`、`kiface/imsgHandler.go`、`kiface/iconnmanager.go`、`kiface/iclient.go`、`kiface/idatapack.go`、`kiface/idecoder.go`、`kiface/ilengthfield.go`、`kiface/iinterceptor.go`、`kiface/iheartbeat.go`、`kiface/irouter.go`

> 删除的僵尸方法：`Inotify`、`IFuncRequest`、`HandleFunc`、`IRequest.Goto`、`IServer.GetLengthField`、`IClient.GetErrChan/SetUrl/GetUrl`、`IDataPack` 的字符串常量、`IFrameDecoder`。

- [ ] **Step 1: 哨兵错误**

`kiface/errors.go`：

```go
// Package kiface defines the contract layer of the Kinz framework.
// Implementations live in knet; business code depends on these interfaces only.
package kiface

import "errors"

// Sentinel errors returned by the framework. Wrap them with %w to preserve identity.
var (
	// ErrServerClosed reports that the server is already shut down.
	ErrServerClosed = errors.New("kinz: server is closed")
	// ErrConnClosed reports that the connection is closed.
	ErrConnClosed = errors.New("kinz: connection is closed")
	// ErrTooLargePacket reports that a packet exceeds the configured max size.
	ErrTooLargePacket = errors.New("kinz: packet exceeds max size")
	// ErrServerFull reports that the server reached its max connection count.
	ErrServerFull = errors.New("kinz: server reached max connections")
	// ErrProtocol reports a malformed or unsupported wire-protocol payload.
	ErrProtocol = errors.New("kinz: protocol error")
	// ErrTimeout reports that an operation exceeded its deadline.
	ErrTimeout = errors.New("kinz: operation timed out")
	// ErrConnNotFound reports that a connection id is not registered.
	ErrConnNotFound = errors.New("kinz: connection not found")
	// ErrMsgIDRegistered reports a duplicate message-id registration.
	ErrMsgIDRegistered = errors.New("kinz: message id already registered")
	// ErrNotImplemented is a placeholder error for methods implemented in later phases.
	ErrNotImplemented = errors.New("kinz: not implemented")
)
```

- [ ] **Step 2: 服务器接口**

`kiface/iserver.go`：

```go
package kiface

import (
	"context"
	"time"
)

// IServer is the top-level server contract. Convention-first: a Server created
// with NewServer() runs with production-safe defaults (heartbeat, max-conn
// rejection, panic recovery, graceful shutdown); extension happens at the seams
// (codec, decoder, interceptors, routers, logger, metrics).
type IServer interface {
	// Run starts accepting connections and blocks until ctx is cancelled or a
	// fatal error occurs. It returns the fatal error, or nil after shutdown.
	Run(ctx context.Context) error
	// Shutdown gracefully stops the server: stop accepting, drain connections,
	// stop the worker pool, and release pooled resources, bounded by ctx.
	Shutdown(ctx context.Context) error
	// Serve runs Run and, once ctx is cancelled, performs a graceful Shutdown.
	Serve(ctx context.Context) error

	// Name returns the server name.
	Name() string

	// AddRouter registers a classic three-stage IRouter for msgID.
	// Returns ErrMsgIDRegistered when msgID is already registered.
	AddRouter(msgID uint32, router IRouter) error
	// AddRouterSlices registers function-style handlers for msgID.
	AddRouterSlices(msgID uint32, handlers ...RouterHandler) (IRouterSlices, error)
	// Group scopes handlers to every msgID in the inclusive range [start, end].
	Group(start, end uint32, handlers ...RouterHandler) (IGroupRouterSlices, error)
	// Use registers global middleware handlers applied to every message.
	Use(handlers ...RouterHandler) (IRouterSlices, error)

	// GetConnMgr returns the connection manager.
	GetConnMgr() IConnManager

	// SetOnConnStart / SetOnConnStop register connection lifecycle hooks.
	SetOnConnStart(func(IConnection))
	SetOnConnStop(func(IConnection))
	GetOnConnStart() func(IConnection)
	GetOnConnStop() func(IConnection)
	// CallOnConnStart / CallOnConnStop invoke the hooks (framework-internal use).
	CallOnConnStart(conn IConnection)
	CallOnConnStop(conn IConnection)

	// SetPacket / GetPacket configure the wire-format pack implementation.
	SetPacket(pack IDataPack)
	GetPacket() IDataPack
	// SetDecoder / GetDecoder configure the frame decoder (TCP sticky/half packets).
	SetDecoder(decoder IDecoder)
	GetDecoder() IDecoder
	// AddInterceptor appends a middleware interceptor to the request pipeline.
	AddInterceptor(interceptor IInterceptor)
	// GetMsgHandler returns the message dispatch module.
	GetMsgHandler() IMsgHandle

	// StartHeartBeat enables heartbeat checking with default options.
	StartHeartBeat(interval time.Duration)
	// SetHeartBeatWithOption enables heartbeat with custom options.
	SetHeartBeatWithOption(interval time.Duration, option *HeartBeatOption)
	// GetHeartBeat returns the heartbeat checker template.
	GetHeartBeat() IHeartbeatChecker
}
```

- [ ] **Step 3: 连接接口**

`kiface/iconnection.go`：

```go
package kiface

import (
	"net"
	"time"
)

// IConnection wraps a single TCP connection with a reader and a writer
// goroutine, liveness tracking, and key-value properties.
type IConnection interface {
	// Start begins the read and write goroutines of this connection.
	Start()
	// Stop gracefully closes the connection. It is idempotent and safe to
	// call from multiple goroutines.
	Stop()
	// GetConn returns the underlying TCP connection.
	GetConn() *net.TCPConn
	// GetConnID returns the connection id (monotonically increasing).
	GetConnID() uint64
	// GetRemoteAddr returns the remote peer address.
	GetRemoteAddr() net.Addr
	// LocalAddr returns the local address of the connection.
	LocalAddr() net.Addr
	// SendMsg packs msgID/data with the server packet format and queues it for
	// writing. It returns ErrConnClosed when the connection is closed.
	SendMsg(msgID uint32, data []byte) error
	// IsAlive reports whether any message was received within timeout.
	IsAlive(timeout time.Duration) bool
	// SetHeartBeat binds a heartbeat checker to this connection.
	SetHeartBeat(hb IHeartbeatChecker)

	// SetProperty attaches a key-value property to the connection.
	SetProperty(key string, value interface{})
	// GetProperty returns the value for key, or an error when absent.
	GetProperty(key string) (interface{}, error)
	// RemoveProperty deletes the property for key.
	RemoveProperty(key string)
}
```

- [ ] **Step 4: 消息与请求接口**

`kiface/imessage.go`：

```go
package kiface

// IMessage is a single protocol message (payload container).
type IMessage interface {
	// GetMsgID returns the message id.
	GetMsgID() uint32
	// GetDataLen returns the payload length in bytes.
	GetDataLen() uint32
	// GetData returns the payload.
	GetData() []byte
	// GetRawData returns the raw bytes seen by the decoder (header when unpacked).
	GetRawData() []byte
	// SetMsgID sets the message id.
	SetMsgID(uint32)
	// SetData sets the payload.
	SetData([]byte)
	// SetDataLen sets the payload length.
	SetDataLen(uint32)
}
```

`kiface/irequest.go`：

```go
package kiface

// IRequest binds a connection and a message, and carries per-request state
// through the router chain (middleware support: Call/Abort, context Set/Get).
type IRequest interface {
	// GetConnection returns the connection that produced this request.
	GetConnection() IConnection
	// GetData returns the message payload.
	GetData() []byte
	// GetMsgID returns the message id.
	GetMsgID() uint32
	// GetMessage returns the underlying message.
	GetMessage() IMessage
	// GetResponse returns the interceptor-chain response (nil when none).
	GetResponse() IcResp
	// SetResponse stores the interceptor-chain response.
	SetResponse(IcResp)

	// BindRouter binds the classic router that handles this request.
	BindRouter(IRouter)
	// Call invokes the bound classic router (PreHandle/Handle/PostHandle).
	Call()
	// Abort stops the remaining function-style handlers; the current one finishes.
	Abort()

	// BindRouterSlices binds the function-style handler chain.
	BindRouterSlices([]RouterHandler)
	// RouterSlicesNext advances to the next function-style handler.
	RouterSlicesNext()

	// Copy returns a shallow copy of the request (worker-pool reuse).
	Copy() IRequest
	// Set stores a value in the request context.
	Set(key string, value interface{})
	// Get reads a value from the request context.
	Get(key string) (interface{}, bool)
}

// BaseRequest is a no-op base for custom IRequest implementations.
type BaseRequest struct{}

func (b BaseRequest) GetConnection() IConnection        { return nil }
func (b BaseRequest) GetData() []byte                    { return nil }
func (b BaseRequest) GetMsgID() uint32                   { return 0 }
func (b BaseRequest) GetMessage() IMessage               { return nil }
func (b BaseRequest) GetResponse() IcResp                { return nil }
func (b BaseRequest) SetResponse(IcResp)                 {}
func (b BaseRequest) BindRouter(IRouter)                 {}
func (b BaseRequest) Call()                              {}
func (b BaseRequest) Abort()                             {}
func (b BaseRequest) BindRouterSlices([]RouterHandler)   {}
func (b BaseRequest) RouterSlicesNext()                  {}
func (b BaseRequest) Copy() IRequest                     { return nil }
func (b BaseRequest) Set(key string, value interface{})  {}
func (b BaseRequest) Get(key string) (interface{}, bool) { return nil, false }
```

- [ ] **Step 5: 消息处理与连接管理器接口**

`kiface/imsgHandler.go`：

```go
package kiface

import "context"

// IMsgHandle dispatches requests to routers and runs the worker pool.
type IMsgHandle interface {
	// AddRouter registers a classic router for msgID.
	// Returns ErrMsgIDRegistered when msgID is already registered.
	AddRouter(msgID uint32, router IRouter) error
	// AddRouterSlices registers function-style handlers for msgID.
	AddRouterSlices(msgID uint32, handlers ...RouterHandler) (IRouterSlices, error)
	// Group scopes handlers to every msgID in [start, end].
	Group(start, end uint32, handlers ...RouterHandler) (IGroupRouterSlices, error)
	// Use registers global middleware applied to every message.
	Use(handlers ...RouterHandler) (IRouterSlices, error)

	// StartWorkerPool launches the worker goroutines (idempotent).
	StartWorkerPool()
	// StopWorkerPool drains and stops the workers, bounded by ctx.
	StopWorkerPool(ctx context.Context)
	// SendMsgToTaskQueue routes a request to a worker by ConnID.
	SendMsgToTaskQueue(request IRequest)
	// Execute runs the interceptor chain, then dispatches to the handler.
	Execute(request IRequest)

	// AddInterceptor appends an interceptor to the chain.
	AddInterceptor(interceptor IInterceptor)
	// SetHeadInterceptor prepends an interceptor to the chain.
	SetHeadInterceptor(interceptor IInterceptor)
}
```

`kiface/iconnmanager.go`：

```go
package kiface

// IConnManager tracks live connections and enforces the max-connection limit.
type IConnManager interface {
	// Add registers a connection. Returns ErrServerFull when the limit is reached.
	Add(conn IConnection) error
	// Remove deregisters a connection.
	Remove(conn IConnection)
	// Get returns the connection with connID, or ErrConnNotFound.
	Get(connID uint64) (IConnection, error)
	// Len returns the number of live connections.
	Len() int
	// ClearConn stops and removes all connections.
	ClearConn()
}
```

- [ ] **Step 6: 客户端接口**

`kiface/iclient.go`：

```go
package kiface

import "time"

// IClient is the TCP client contract (full implementation lands in Phase 3).
type IClient interface {
	// Start connects to the server and blocks until Stop is called.
	// Returns the connect error (with retries applied by the implementation).
	Start() error
	// Stop disconnects and disables auto-reconnect.
	Stop()
	// Restart stops the current session and starts a new one.
	Restart()

	// Conn returns the current connection (nil when disconnected).
	Conn() IConnection
	// AddRouter registers a classic router for msgID.
	AddRouter(msgID uint32, router IRouter) error

	// SetOnConnStart / SetOnConnStop register connection lifecycle hooks.
	SetOnConnStart(func(IConnection))
	SetOnConnStop(func(IConnection))
	GetOnConnStart() func(IConnection)
	GetOnConnStop() func(IConnection)

	// SetPacket / GetPacket configure the wire-format pack implementation.
	SetPacket(IDataPack)
	GetPacket() IDataPack
	// SetDecoder configures the frame decoder.
	SetDecoder(IDecoder)
	// AddInterceptor appends a middleware interceptor.
	AddInterceptor(IInterceptor)
	// GetMsgHandler returns the message dispatch module.
	GetMsgHandler() IMsgHandle

	// StartHeartBeat enables heartbeat sending with default options.
	StartHeartBeat(interval time.Duration)
	// StartHeartBeatWithOption enables heartbeat with custom options.
	StartHeartBeatWithOption(interval time.Duration, option *HeartBeatOption)

	// SetName / GetName manage the client name.
	SetName(string)
	GetName() string
}
```

- [ ] **Step 7: 编解码器接口**

`kiface/idatapack.go`：

```go
package kiface

// IDataPack packs and unpacks messages in a TLV wire format.
type IDataPack interface {
	// GetHeadLen returns the header length in bytes.
	GetHeadLen() uint32
	// Pack serializes msg into wire-format bytes.
	Pack(msg IMessage) ([]byte, error)
	// Unpack parses the header from binaryData and returns a message with the
	// id and data length set; the payload must be read separately.
	Unpack(binaryData []byte) (IMessage, error)
}
```

`kiface/idecoder.go`：

```go
package kiface

// IDecoder converts a raw byte stream into complete frames, handling TCP
// sticky and half packets. Stateful: a single instance serves one connection.
type IDecoder interface {
	// Decode consumes buffered bytes and returns complete frames.
	// It returns (nil, nil) when more data is needed for a full frame.
	// Protocol errors are returned as errors (Phase 2 converts FrameDecoder).
	Decode(buff []byte) [][]byte
	// GetLengthField returns the length-field configuration of this decoder.
	GetLengthField() *LengthField
}
```

`kiface/ilengthfield.go`：

```go
package kiface

import "encoding/binary"

// LengthField describes how to locate the frame length in a byte stream.
type LengthField struct {
	// Order is the byte order of the length field (default big-endian).
	Order binary.ByteOrder
	// MaxFrameLength caps a single frame; larger frames are treated as errors.
	MaxFrameLength uint64
	// LengthFieldOffset is the offset of the length field in the frame.
	LengthFieldOffset int
	// LengthFieldLength is the byte width of the length field (1/2/3/4/8).
	LengthFieldLength int
	// LengthAdjustment is added to the length-field value.
	LengthAdjustment int
	// InitialBytesToStrip is how many leading bytes to drop from each frame.
	InitialBytesToStrip int
}
```

`kiface/iinterceptor.go`：

```go
package kiface

// IcReq is the interceptor-chain input (any value).
type IcReq interface{}

// IcResp is the interceptor-chain output (any value).
type IcResp interface{}

// IInterceptor is one step of the request pipeline (responsibility chain).
type IInterceptor interface {
	// Intercept processes the chain request; call chain.Proceed to continue.
	Intercept(chain IChain) IcResp
}

// IChain is the responsibility chain over interceptors.
type IChain interface {
	// Request returns the current request payload.
	Request() IcReq
	// GetIMessage returns the IMessage inside the current request, if any.
	GetIMessage() IMessage
	// Proceed advances to the next interceptor with req.
	Proceed(req IcReq) IcResp
	// ProceedWithIMessage replaces the message and advances.
	ProceedWithIMessage(iMessage IMessage, response IcReq) IcResp
}
```

- [ ] **Step 8: 心跳与路由接口**

`kiface/iheartbeat.go`：

```go
package kiface

// HeartBeatDefaultMsgID is the default heartbeat message id.
const HeartBeatDefaultMsgID uint32 = 99999

// HeartBeatMsgFunc builds the payload of a heartbeat message.
type HeartBeatMsgFunc func(conn IConnection) []byte

// HeartBeatFunc sends a heartbeat message; a non-nil error marks the peer dead.
type HeartBeatFunc func(conn IConnection) error

// OnRemoteNotAlive handles a peer that did not stay alive.
type OnRemoteNotAlive func(conn IConnection)

// HeartBeatOption customizes heartbeat behavior.
type HeartBeatOption struct {
	MakeMsg          HeartBeatMsgFunc
	OnRemoteNotAlive OnRemoteNotAlive
	HeartBeatMsgID   uint32
	Router           IRouter
	IRouterSlices    []RouterHandler
}

// IHeartbeatChecker tracks liveness of one connection and sends heartbeats.
type IHeartbeatChecker interface {
	SetOnRemoteNotAlive(OnRemoteNotAlive)
	SetHeartBeatMsgFunc(HeartBeatMsgFunc)
	SetHeartbeatFunc(HeartBeatFunc)
	BindRouter(msgID uint32, router IRouter)
	BindRouterSlices(msgID uint32, handlers ...RouterHandler)
	Start()
	Stop()
	SendHeartBeatMsg() error
	BindConn(conn IConnection)
	Clone() IHeartbeatChecker
	MsgID() uint32
	Router() IRouter
	RouterSlices() []RouterHandler
}
```

`kiface/irouter.go`：

```go
package kiface

// IRouter is the classic three-stage router. Embed BaseRouter in
// implementations and override only the methods you need.
type IRouter interface {
	// PreHandle runs before Handle.
	PreHandle(request IRequest)
	// Handle runs the main business logic.
	Handle(request IRequest)
	// PostHandle runs after Handle.
	PostHandle(request IRequest)
}

// RouterHandler is a function-style message handler.
type RouterHandler func(request IRequest)

// IRouterSlices is the function-style router with middleware support.
type IRouterSlices interface {
	// Use appends global middleware handlers and returns the router for chaining.
	Use(handlers ...RouterHandler) IRouterSlices
	// AddHandler registers handlers for msgID.
	// Returns ErrMsgIDRegistered when msgID is already registered.
	AddHandler(msgID uint32, handlers ...RouterHandler) error
	// Group scopes handlers to every msgID in [start, end].
	Group(start, end uint32, handlers ...RouterHandler) IGroupRouterSlices
	// GetHandlers returns the handlers registered for msgID.
	GetHandlers(msgID uint32) ([]RouterHandler, bool)
}

// IGroupRouterSlices is a router scoped to a msgID range.
type IGroupRouterSlices interface {
	// Use appends middleware scoped to the group.
	Use(handlers ...RouterHandler) IGroupRouterSlices
	// AddHandler registers handlers for a msgID inside the group range.
	AddHandler(msgID uint32, handlers ...RouterHandler) error
}
```

- [ ] **Step 9: 验证 kiface 可编译**

Run：

```powershell
go build ./kiface/
```

Expected: 成功，无输出（此时全仓仍是 broken，属预期）。

- [ ] **Step 10: 提交**

```bash
git add kiface
git commit -m "refactor(kiface): redefine contract layer per convention-first philosophy"
```

**Task 2 结束时的构建状态：预期 broken（knet/datapack.go 引用已删除的 utils 与旧方法名 GetMsgId）。**

---

## Task 3: klog 重写（log/slog）

> 用户要求将 klog 重写提前到 P1（原计划 P3）。`log/slog` 为标准库结构化日志，性能优于旧 zlog（惰性格式化、零分配路径、无硬编码颜色）。

**Files:**
- Rewrite: `klog/ilog.go`
- Rewrite: `klog/log.go`
- Create: `klog/log_test.go`

- [ ] **Step 1: 写失败测试（定义目标 API）**

创建 `klog/log_test.go`：

```go
package klog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultLogger(t *testing.T) {
	if L() == nil {
		t.Fatal("L() returned nil")
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, Level: LevelError})

	lg.Info("should be filtered")
	lg.Error("should be emitted")

	if strings.Contains(buf.String(), "should be filtered") {
		t.Fatal("Info was emitted despite LevelError")
	}
	if !strings.Contains(buf.String(), "should be emitted") {
		t.Fatalf("Error not emitted, got: %q", buf.String())
	}
}

func TestLevelVarDynamic(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, Level: LevelInfo})
	lg.SetLevel(LevelWarn)
	lg.Info("info line")
	if strings.Contains(buf.String(), "info line") {
		t.Fatal("Info emitted after raising level to Warn")
	}
}

func TestJSONHandler(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, JSON: true})

	lg.Info("hello", "key", "value")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v; got %q", err, buf.String())
	}
	if rec["msg"] != "hello" {
		t.Fatalf("msg = %v, want hello", rec["msg"])
	}
	if rec["key"] != "value" {
		t.Fatalf("key = %v, want value", rec["key"])
	}
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, JSON: true})

	lg.With("connID", 42).Info("conn event")

	var rec map[string]any
	_ = json.Unmarshal(buf.Bytes(), &rec)
	if rec["connID"] != float64(42) {
		t.Fatalf("connID = %v, want 42", rec["connID"])
	}
}

func TestInfofCompatibility(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, JSON: true})

	lg.InfoF("ping %d", 1)

	var rec map[string]any
	_ = json.Unmarshal(buf.Bytes(), &rec)
	if rec["msg"] != "ping 1" {
		t.Fatalf("msg = %v, want 'ping 1'", rec["msg"])
	}
}
```

- [ ] **Step 2: 运行测试，确认失败（API 不存在）**

Run:

```powershell
go test ./klog/ -v
```

Expected: 编译失败——`Options`、`New`、`LevelError` 等未定义（旧 klog 无此 API）。预期红。

- [ ] **Step 3: 实现 ILogger 接口**

重写 `klog/ilog.go`：

```go
// Package klog provides the Kinz logging contract and a log/slog-based
// default implementation. It is the logging seam: business code can inject
// any ILogger via klog.SetDefault or a server option.
package klog

import "log/slog"

// Level is a log severity level (alias of slog.Level).
type Level = slog.Level

// Predefined severity levels.
const (
	LevelDebug = slog.LevelDebug // -4
	LevelInfo  = slog.LevelInfo  // 0
	LevelWarn  = slog.LevelWarn  // 4
	LevelError = slog.LevelError // 8
)

// ILogger is the framework's logging contract.
type ILogger interface {
	// Debug logs at debug level.
	Debug(msg string, args ...any)
	// Info logs at info level.
	Info(msg string, args ...any)
	// Warn logs at warn level.
	Warn(msg string, args ...any)
	// Error logs at error level.
	Error(msg string, args ...any)
	// InfoF logs a printf-formatted message at info level (legacy compatibility).
	InfoF(format string, args ...any)
	// ErrorF logs a printf-formatted message at error level (legacy compatibility).
	ErrorF(format string, args ...any)
	// With returns a logger with structured fields attached.
	With(fields ...any) ILogger
}
```

- [ ] **Step 4: 实现 slog 封装**

重写 `klog/log.go`：

```go
package klog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Options configures a Logger.
type Options struct {
	// Level is the minimum severity to emit (default Info).
	Level Level
	// JSON selects the JSON handler instead of the text handler.
	JSON bool
	// AddSource includes the caller file:line in each record.
	AddSource bool
	// Output is the destination (default os.Stdout).
	Output io.Writer
}

// Logger is the default ILogger implementation backed by log/slog.
// Its level is dynamic: SetLevel takes effect immediately.
type Logger struct {
	l        *slog.Logger
	levelVar *slog.LevelVar
}

// New creates a Logger from opts.
func New(opts Options) *Logger {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	levelVar := new(slog.LevelVar)
	levelVar.Set(slog.Level(opts.Level))
	handlerOpts := &slog.HandlerOptions{Level: levelVar, AddSource: opts.AddSource}

	var handler slog.Handler
	if opts.JSON {
		handler = slog.NewJSONHandler(opts.Output, handlerOpts)
	} else {
		handler = slog.NewTextHandler(opts.Output, handlerOpts)
	}
	return &Logger{l: slog.New(handler), levelVar: levelVar}
}

// SetLevel adjusts the minimum severity dynamically.
func (lg *Logger) SetLevel(level Level) { lg.levelVar.Set(slog.Level(level)) }

// Debug implements ILogger.
func (lg *Logger) Debug(msg string, args ...any) { lg.l.Debug(msg, args...) }

// Info implements ILogger.
func (lg *Logger) Info(msg string, args ...any) { lg.l.Info(msg, args...) }

// Warn implements ILogger.
func (lg *Logger) Warn(msg string, args ...any) { lg.l.Warn(msg, args...) }

// Error implements ILogger.
func (lg *Logger) Error(msg string, args ...any) { lg.l.Error(msg, args...) }

// InfoF implements ILogger.
func (lg *Logger) InfoF(format string, args ...any) {
	lg.l.Info(fmt.Sprintf(format, args...))
}

// ErrorF implements ILogger.
func (lg *Logger) ErrorF(format string, args ...any) {
	lg.l.Error(fmt.Sprintf(format, args...))
}

// With implements ILogger.
func (lg *Logger) With(fields ...any) ILogger {
	return &Logger{l: lg.l.With(fields...), levelVar: lg.levelVar}
}

var defaultLogger = New(Options{})

// L returns the package-level default logger.
func L() ILogger { return defaultLogger }

// SetDefault replaces the package-level default logger.
func SetDefault(l ILogger) { defaultLogger = l }

// Package-level convenience delegates to L().
func Debug(msg string, args ...any) { L().Debug(msg, args...) }
func Info(msg string, args ...any)  { L().Info(msg, args...) }
func Warn(msg string, args ...any)  { L().Warn(msg, args...) }
func Error(msg string, args ...any) { L().Error(msg, args...) }
func InfoF(format string, args ...any) { L().InfoF(format, args...) }
func ErrorF(format string, args ...any) { L().ErrorF(format, args...) }
```

- [ ] **Step 5: 运行测试，确认通过**

Run:

```powershell
go test ./klog/ -v
```

Expected: 6 个测试全部 PASS。

- [ ] **Step 6: 提交**

```bash
git add klog
git commit -m "refactor(klog): rewrite with log/slog (levels, JSON, dynamic level)"
```

**Task 3 结束时的构建状态：klog 独立可编译、测试通过；全仓仍 broken（knet/datapack.go 引用已删除的 utils）。**

---

## Task 4: knet 编解码层迁移（TLV + 单元测试）

**Files:**
- Rewrite: `knet/message.go`
- Rewrite: `knet/datapack.go`
- Rewrite: `knet/datapack_test.go`（替换旧手工脚本）

- [ ] **Step 1: 写失败测试（定义目标 API）**

> 覆盖率门禁要求：除往返/超限/短头外，追加 `TestMessageSetters` 与 `GetRawData` 断言，使 `message.go`/`datapack.go` 行覆盖 ≥ 80%。

创建 `knet/datapack_test.go`：

```go
package knet

import (
	"bytes"
	"errors"
	"testing"

	"kinz/kiface"
)

func TestDataPackRoundTrip(t *testing.T) {
	dp := NewDataPack()
	msg := NewMessage(42, []byte("hello kinz"))

	wire, err := dp.Pack(msg)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(wire) != int(dp.GetHeadLen())+len("hello kinz") {
		t.Fatalf("wire len = %d, want %d", len(wire), dp.GetHeadLen()+uint32(len("hello kinz")))
	}

	head := wire[:dp.GetHeadLen()]
	unpacked, err := dp.Unpack(head)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if unpacked.GetMsgID() != 42 {
		t.Fatalf("MsgID = %d, want 42", unpacked.GetMsgID())
	}
	if unpacked.GetDataLen() != uint32(len("hello kinz")) {
		t.Fatalf("DataLen = %d, want %d", unpacked.GetDataLen(), len("hello kinz"))
	}
	if !bytes.Equal(unpacked.GetRawData(), head) {
		t.Fatalf("Raw = %q, want head %q", unpacked.GetRawData(), head)
	}
	if !bytes.Equal(wire[dp.GetHeadLen():], []byte("hello kinz")) {
		t.Fatalf("payload = %q, want %q", wire[dp.GetHeadLen():], "hello kinz")
	}
}

func TestMessageSetters(t *testing.T) {
	msg := NewMessage(1, nil)
	msg.SetMsgID(7)
	msg.SetData([]byte("xyz"))
	msg.SetDataLen(2)

	if msg.GetMsgID() != 7 {
		t.Fatalf("MsgID = %d, want 7", msg.GetMsgID())
	}
	if msg.GetDataLen() != 2 {
		t.Fatalf("DataLen = %d, want 2", msg.GetDataLen())
	}
	if !bytes.Equal(msg.GetData(), []byte("xyz")) {
		t.Fatalf("Data = %q, want xyz", msg.GetData())
	}
	if msg.GetRawData() != nil {
		t.Fatal("Raw should be nil for a fresh message")
	}
}

func TestDataPackRejectsOversize(t *testing.T) {
	// DataLen = 0x00100000 (little-endian), MsgID = 1.
	head := []byte{0x00, 0x00, 0x10, 0x00, 0x01, 0x00, 0x00, 0x00}
	if _, err := NewDataPack().Unpack(head); err == nil {
		t.Fatal("expected error for oversize packet")
	} else if !errors.Is(err, kiface.ErrTooLargePacket) {
		t.Fatalf("err = %v, want ErrTooLargePacket", err)
	}
}

func TestDataPackRejectsShortHeader(t *testing.T) {
	if _, err := NewDataPack().Unpack([]byte{0x01}); err == nil {
		t.Fatal("expected error for short header")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败（API 未实现）**

Run：

```powershell
go test ./knet/ -run TestDataPack -v
```

Expected: 编译失败——`Message` 无 `GetMsgID`（旧为 `GetMsgId`）、`NewMessage`/`NewDataPack` 行为不符或 `utils` 缺失。此为预期红。

- [ ] **Step 3: 实现 Message**

重写 `knet/message.go`：

```go
package knet

// Message is the default IMessage implementation: a TLV payload container.
// Fields are exported for direct decoder use (binary.Read needs field pointers).
type Message struct {
	Id      uint32
	DataLen uint32
	Data    []byte
	Raw     []byte
}

// NewMessage creates a Message with id and payload (DataLen derived).
func NewMessage(id uint32, data []byte) *Message {
	return &Message{Id: id, DataLen: uint32(len(data)), Data: data}
}

// GetMsgID returns the message id.
func (m *Message) GetMsgID() uint32 { return m.Id }

// GetDataLen returns the payload length.
func (m *Message) GetDataLen() uint32 { return m.DataLen }

// GetData returns the payload.
func (m *Message) GetData() []byte { return m.Data }

// GetRawData returns the raw header bytes captured during Unpack.
func (m *Message) GetRawData() []byte { return m.Raw }

// SetMsgID sets the message id.
func (m *Message) SetMsgID(id uint32) { m.Id = id }

// SetData sets the payload (DataLen is set by the decoder from the header).
func (m *Message) SetData(data []byte) { m.Data = data }

// SetDataLen sets the payload length.
func (m *Message) SetDataLen(l uint32) { m.DataLen = l }
```

- [ ] **Step 4: 实现 DataPack**

重写 `knet/datapack.go`：

```go
package knet

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"kinz/kiface"
)

// defaultMaxPacketSize caps a single packet payload when no config is set
// (matches the historical default; Phase 2 wires it to kconf).
const defaultMaxPacketSize uint32 = 4096

// DataPack implements the default TLV wire format (little-endian):
// [DataLen:4][MsgID:4][Data:DataLen].
type DataPack struct {
	maxPacketSize uint32
}

// NewDataPack returns a DataPack with the default max packet size.
func NewDataPack() *DataPack {
	return &DataPack{maxPacketSize: defaultMaxPacketSize}
}

// GetHeadLen returns the header length in bytes.
func (dp *DataPack) GetHeadLen() uint32 { return 8 }

// Pack serializes msg into wire-format bytes.
func (dp *DataPack) Pack(msg kiface.IMessage) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, dp.GetHeadLen()+msg.GetDataLen()))
	if err := binary.Write(buf, binary.LittleEndian, msg.GetDataLen()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, msg.GetMsgID()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, msg.GetData()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unpack parses the header and returns a Message with id/dataLen set;
// the payload must be read separately (see Phase 2 pipeline).
func (dp *DataPack) Unpack(binaryData []byte) (kiface.IMessage, error) {
	if len(binaryData) < int(dp.GetHeadLen()) {
		return nil, fmt.Errorf("%w: header needs %d bytes, got %d",
			kiface.ErrProtocol, dp.GetHeadLen(), len(binaryData))
	}
	reader := bytes.NewReader(binaryData[:dp.GetHeadLen()])
	msg := &Message{}
	if err := binary.Read(reader, binary.LittleEndian, &msg.DataLen); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &msg.Id); err != nil {
		return nil, err
	}
	if dp.maxPacketSize > 0 && msg.DataLen > dp.maxPacketSize {
		return nil, fmt.Errorf("%w: got %d bytes, max %d",
			kiface.ErrTooLargePacket, msg.DataLen, dp.maxPacketSize)
	}
	msg.Raw = append([]byte(nil), binaryData[:dp.GetHeadLen()]...)
	return msg, nil
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run：

```powershell
go build ./...
go test ./knet/ -run TestDataPack -v
```

Expected: `go build ./...` **首次全绿**；三个测试全部 PASS。

- [ ] **Step 6: 覆盖率验证（P1 门禁）**

Run：

```powershell
go test ./knet/ -coverprofile=cover.out
go tool cover -func=cover.out | Select-String -Pattern 'datapack.go|message.go|total'
```

Expected: `datapack.go`、`message.go` 行覆盖率 **≥ 80%**，total 供记录（桩代码会拉低 total，属预期）。

**Task 4 追加（用户反馈）**：大小端通用化——`DataPack` 增加 `order binary.ByteOrder` 字段与 `NewDataPackWithOrder(order)` 构造函数（nil 回退小端），默认保持小端以兼容旧线协议；P2 由 kconf 配置。新增 `TestDataPackBigEndian` 验证大端封解包与线格式字节序。原则：字节序是线协议决策，显式声明、可配置，不做运行时主机字节序探测。

- [ ] **Step 7: 提交**

```bash
git add knet/message.go knet/datapack.go knet/datapack_test.go
git commit -m "feat(knet): migrate TLV codec with unit tests and sentinel errors"
```

**Task 3 结束时的构建状态：`go build ./...` 首次转绿。**

---

## Task 5: knet 桩实现 + 接口断言

**Files（全部重写为满足接口的桩，P2 重写行为）：**
- Rewrite: `knet/server.go`、`knet/connection.go`、`knet/msgHandler.go`、`knet/connmanager.go`、`knet/heartbeat.go`、`knet/client.go`
- Create: `knet/interface_test.go`、`kinterceptor/interface_test.go`

- [ ] **Step 1: Server 桩**

`knet/server.go`：

```go
package knet

import (
	"context"
	"time"

	"kinz/kiface"
)

// Option customizes a Server at construction time.
type Option func(*Server)

// Server implements kiface.IServer. Behavioral implementation lands in Phase 2.
type Server struct {
	name       string
	connMgr    kiface.IConnManager
	msgHandler kiface.IMsgHandle
	packet     kiface.IDataPack
	decoder    kiface.IDecoder

	onConnStart func(kiface.IConnection)
	onConnStop  func(kiface.IConnection)
	heartbeat   kiface.IHeartbeatChecker
}

// NewServer creates a Server with default configuration and applies opts.
func NewServer(opts ...Option) kiface.IServer {
	s := &Server{
		name:       "KinzServer",
		connMgr:    NewConnManager(0),
		msgHandler: NewMsgHandler(),
		packet:     NewDataPack(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Run implements kiface.IServer. Phase 2.
func (s *Server) Run(ctx context.Context) error { return kiface.ErrNotImplemented }

// Shutdown implements kiface.IServer. Phase 2.
func (s *Server) Shutdown(ctx context.Context) error { return nil }

// Serve implements kiface.IServer. Phase 2.
func (s *Server) Serve(ctx context.Context) error { return kiface.ErrNotImplemented }

// Name implements kiface.IServer.
func (s *Server) Name() string { return s.name }

// AddRouter implements kiface.IServer. Phase 2.
func (s *Server) AddRouter(msgID uint32, router kiface.IRouter) error {
	return kiface.ErrNotImplemented
}

// AddRouterSlices implements kiface.IServer. Phase 2.
func (s *Server) AddRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// Group implements kiface.IServer. Phase 2.
func (s *Server) Group(start, end uint32, handlers ...kiface.RouterHandler) (kiface.IGroupRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// Use implements kiface.IServer. Phase 2.
func (s *Server) Use(handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// GetConnMgr implements kiface.IServer.
func (s *Server) GetConnMgr() kiface.IConnManager { return s.connMgr }

// SetOnConnStart implements kiface.IServer.
func (s *Server) SetOnConnStart(f func(kiface.IConnection)) { s.onConnStart = f }

// SetOnConnStop implements kiface.IServer.
func (s *Server) SetOnConnStop(f func(kiface.IConnection)) { s.onConnStop = f }

// GetOnConnStart implements kiface.IServer.
func (s *Server) GetOnConnStart() func(kiface.IConnection) { return s.onConnStart }

// GetOnConnStop implements kiface.IServer.
func (s *Server) GetOnConnStop() func(kiface.IConnection) { return s.onConnStop }

// CallOnConnStart implements kiface.IServer.
func (s *Server) CallOnConnStart(conn kiface.IConnection) {
	if s.onConnStart != nil {
		s.onConnStart(conn)
	}
}

// CallOnConnStop implements kiface.IServer.
func (s *Server) CallOnConnStop(conn kiface.IConnection) {
	if s.onConnStop != nil {
		s.onConnStop(conn)
	}
}

// SetPacket implements kiface.IServer.
func (s *Server) SetPacket(pack kiface.IDataPack) { s.packet = pack }

// GetPacket implements kiface.IServer.
func (s *Server) GetPacket() kiface.IDataPack { return s.packet }

// SetDecoder implements kiface.IServer.
func (s *Server) SetDecoder(decoder kiface.IDecoder) { s.decoder = decoder }

// GetDecoder implements kiface.IServer.
func (s *Server) GetDecoder() kiface.IDecoder { return s.decoder }

// AddInterceptor implements kiface.IServer.
func (s *Server) AddInterceptor(interceptor kiface.IInterceptor) {
	s.msgHandler.AddInterceptor(interceptor)
}

// GetMsgHandler implements kiface.IServer.
func (s *Server) GetMsgHandler() kiface.IMsgHandle { return s.msgHandler }

// StartHeartBeat implements kiface.IServer. Phase 2.
func (s *Server) StartHeartBeat(interval time.Duration) {}

// SetHeartBeatWithOption implements kiface.IServer. Phase 2.
func (s *Server) SetHeartBeatWithOption(interval time.Duration, option *kiface.HeartBeatOption) {}

// GetHeartBeat implements kiface.IServer.
func (s *Server) GetHeartBeat() kiface.IHeartbeatChecker { return s.heartbeat }
```

- [ ] **Step 2: Connection 桩**

`knet/connection.go`：

```go
package knet

import (
	"errors"
	"net"
	"sync"
	"time"

	"kinz/kiface"
)

// Connection implements kiface.IConnection. Behavioral implementation lands in Phase 2.
type Connection struct {
	conn   *net.TCPConn
	connID uint64

	property     map[string]interface{}
	propertyLock sync.RWMutex
}

// NewConnection wraps a TCP connection. Lifecycle registration lands in Phase 2.
func NewConnection(conn *net.TCPConn, connID uint64) *Connection {
	return &Connection{
		conn:     conn,
		connID:   connID,
		property: make(map[string]interface{}),
	}
}

// Start implements kiface.IConnection. Phase 2.
func (c *Connection) Start() {}

// Stop implements kiface.IConnection. Phase 2 (graceful, idempotent).
func (c *Connection) Stop() { _ = c.conn.Close() }

// GetConn implements kiface.IConnection.
func (c *Connection) GetConn() *net.TCPConn { return c.conn }

// GetConnID implements kiface.IConnection.
func (c *Connection) GetConnID() uint64 { return c.connID }

// GetRemoteAddr implements kiface.IConnection.
func (c *Connection) GetRemoteAddr() net.Addr { return c.conn.RemoteAddr() }

// LocalAddr implements kiface.IConnection.
func (c *Connection) LocalAddr() net.Addr { return c.conn.LocalAddr() }

// SendMsg implements kiface.IConnection. Phase 2.
func (c *Connection) SendMsg(msgID uint32, data []byte) error { return kiface.ErrNotImplemented }

// IsAlive implements kiface.IConnection. Phase 2.
func (c *Connection) IsAlive(timeout time.Duration) bool { return true }

// SetHeartBeat implements kiface.IConnection. Phase 2.
func (c *Connection) SetHeartBeat(hb kiface.IHeartbeatChecker) {}

// SetProperty implements kiface.IConnection.
func (c *Connection) SetProperty(key string, value interface{}) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()
	c.property[key] = value
}

// GetProperty implements kiface.IConnection.
func (c *Connection) GetProperty(key string) (interface{}, error) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()
	if v, ok := c.property[key]; ok {
		return v, nil
	}
	return nil, errors.New("kinz: no property found")
}

// RemoveProperty implements kiface.IConnection.
func (c *Connection) RemoveProperty(key string) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()
	delete(c.property, key)
}
```

- [ ] **Step 3: MsgHandler 桩**

`knet/msgHandler.go`：

```go
package knet

import (
	"context"

	"kinz/kiface"
)

// MsgHandler implements kiface.IMsgHandle. Behavioral implementation lands in Phase 2.
type MsgHandler struct{}

// NewMsgHandler creates a MsgHandler.
func NewMsgHandler() *MsgHandler { return &MsgHandler{} }

// AddRouter implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) AddRouter(msgID uint32, router kiface.IRouter) error {
	return kiface.ErrNotImplemented
}

// AddRouterSlices implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) AddRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// Group implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) Group(start, end uint32, handlers ...kiface.RouterHandler) (kiface.IGroupRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// Use implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) Use(handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// StartWorkerPool implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) StartWorkerPool() {}

// StopWorkerPool implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) StopWorkerPool(ctx context.Context) {}

// SendMsgToTaskQueue implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) SendMsgToTaskQueue(request kiface.IRequest) {}

// Execute implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) Execute(request kiface.IRequest) {}

// AddInterceptor implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) AddInterceptor(interceptor kiface.IInterceptor) {}

// SetHeadInterceptor implements kiface.IMsgHandle. Phase 2.
func (mh *MsgHandler) SetHeadInterceptor(interceptor kiface.IInterceptor) {}
```

- [ ] **Step 4: ConnManager（近乎最终形态）**

`knet/connmanager.go`：

```go
package knet

import (
	"sync"

	"kinz/kiface"
)

// ConnManager implements kiface.IConnManager with a mutex-protected map and a
// max-connection limit (maxConn <= 0 means unlimited).
type ConnManager struct {
	mu          sync.RWMutex
	connections map[uint64]kiface.IConnection
	maxConn     int
}

// NewConnManager creates a ConnManager with the given limit.
func NewConnManager(maxConn int) *ConnManager {
	return &ConnManager{
		connections: make(map[uint64]kiface.IConnection),
		maxConn:     maxConn,
	}
}

// Add implements kiface.IConnManager. Returns ErrServerFull at the limit.
func (cm *ConnManager) Add(conn kiface.IConnection) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.maxConn > 0 && len(cm.connections) >= cm.maxConn {
		return kiface.ErrServerFull
	}
	cm.connections[conn.GetConnID()] = conn
	return nil
}

// Remove implements kiface.IConnManager.
func (cm *ConnManager) Remove(conn kiface.IConnection) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.connections, conn.GetConnID())
}

// Get implements kiface.IConnManager.
func (cm *ConnManager) Get(connID uint64) (kiface.IConnection, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	if conn, ok := cm.connections[connID]; ok {
		return conn, nil
	}
	return nil, kiface.ErrConnNotFound
}

// Len implements kiface.IConnManager.
func (cm *ConnManager) Len() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.connections)
}

// ClearConn implements kiface.IConnManager. Stops connections outside the lock
// so Stop() may call Remove() without deadlocking.
func (cm *ConnManager) ClearConn() {
	cm.mu.Lock()
	conns := make([]kiface.IConnection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		conns = append(conns, conn)
	}
	cm.connections = make(map[uint64]kiface.IConnection)
	cm.mu.Unlock()
	for _, conn := range conns {
		conn.Stop()
	}
}
```

- [ ] **Step 5: HeartBeatChecker 桩**

`knet/heartbeat.go`：

```go
package knet

import "kinz/kiface"

// HeartBeatChecker implements kiface.IHeartbeatChecker.
// Behavioral implementation lands in Phase 2.
type HeartBeatChecker struct{}

// NewHeartbeatChecker creates a HeartBeatChecker.
func NewHeartbeatChecker() *HeartBeatChecker { return &HeartBeatChecker{} }

// SetOnRemoteNotAlive implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) SetOnRemoteNotAlive(f kiface.OnRemoteNotAlive) {}

// SetHeartBeatMsgFunc implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) SetHeartBeatMsgFunc(f kiface.HeartBeatMsgFunc) {}

// SetHeartbeatFunc implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) SetHeartbeatFunc(f kiface.HeartBeatFunc) {}

// BindRouter implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) BindRouter(msgID uint32, router kiface.IRouter) {}

// BindRouterSlices implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) BindRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) {}

// Start implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) Start() {}

// Stop implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) Stop() {}

// SendHeartBeatMsg implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) SendHeartBeatMsg() error { return kiface.ErrNotImplemented }

// BindConn implements kiface.IHeartbeatChecker. Phase 2.
func (h *HeartBeatChecker) BindConn(conn kiface.IConnection) {}

// Clone implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) Clone() kiface.IHeartbeatChecker { return NewHeartbeatChecker() }

// MsgID implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) MsgID() uint32 { return kiface.HeartBeatDefaultMsgID }

// Router implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) Router() kiface.IRouter { return nil }

// RouterSlices implements kiface.IHeartbeatChecker.
func (h *HeartBeatChecker) RouterSlices() []kiface.RouterHandler { return nil }
```

- [ ] **Step 6: Client 桩**

`knet/client.go`：

```go
package knet

import (
	"time"

	"kinz/kiface"
)

// Client implements kiface.IClient. Behavioral implementation lands in Phase 3.
type Client struct {
	name string
}

// NewClient creates a Client and applies opts. Full client lands in Phase 3.
func NewClient(ip string, port int, opts ...ClientOption) kiface.IClient {
	c := &Client{name: "KinzClient"}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Start implements kiface.IClient. Phase 3.
func (c *Client) Start() error { return kiface.ErrNotImplemented }

// Stop implements kiface.IClient. Phase 3.
func (c *Client) Stop() {}

// Restart implements kiface.IClient. Phase 3.
func (c *Client) Restart() {}

// Conn implements kiface.IClient. Phase 3.
func (c *Client) Conn() kiface.IConnection { return nil }

// AddRouter implements kiface.IClient. Phase 3.
func (c *Client) AddRouter(msgID uint32, router kiface.IRouter) error {
	return kiface.ErrNotImplemented
}

// SetOnConnStart implements kiface.IClient. Phase 3.
func (c *Client) SetOnConnStart(f func(kiface.IConnection)) {}

// SetOnConnStop implements kiface.IClient. Phase 3.
func (c *Client) SetOnConnStop(f func(kiface.IConnection)) {}

// GetOnConnStart implements kiface.IClient.
func (c *Client) GetOnConnStart() func(kiface.IConnection) { return nil }

// GetOnConnStop implements kiface.IClient.
func (c *Client) GetOnConnStop() func(kiface.IConnection) { return nil }

// SetPacket implements kiface.IClient.
func (c *Client) SetPacket(pack kiface.IDataPack) {}

// GetPacket implements kiface.IClient.
func (c *Client) GetPacket() kiface.IDataPack { return nil }

// SetDecoder implements kiface.IClient. Phase 3.
func (c *Client) SetDecoder(decoder kiface.IDecoder) {}

// AddInterceptor implements kiface.IClient. Phase 3.
func (c *Client) AddInterceptor(interceptor kiface.IInterceptor) {}

// GetMsgHandler implements kiface.IClient. Phase 3.
func (c *Client) GetMsgHandler() kiface.IMsgHandle { return nil }

// StartHeartBeat implements kiface.IClient. Phase 3.
func (c *Client) StartHeartBeat(interval time.Duration) {}

// StartHeartBeatWithOption implements kiface.IClient. Phase 3.
func (c *Client) StartHeartBeatWithOption(interval time.Duration, option *kiface.HeartBeatOption) {}

// SetName implements kiface.IClient.
func (c *Client) SetName(name string) { c.name = name }

// GetName implements kiface.IClient.
func (c *Client) GetName() string { return c.name }
```

- [ ] **Step 7: 补全 FrameDecoder 的 GetLengthField 方法**

`kinterceptor/framedecoder.go` 目前缺少 `GetLengthField()`（旧接口要求但实现缺失，属接口漂移）。在 `NewFrameDecoder` 函数之后追加：

```go
// GetLengthField returns the length-field configuration of this decoder.
func (d *FrameDecoder) GetLengthField() *kiface.LengthField {
	return &d.LengthField
}
```

（该文件其余部分保持不变；内部 panic 的错误化改造在 P2。）

- [ ] **Step 8: 接口断言测试（编译期契约）**

创建 `knet/interface_test.go`：

```go
package knet

import "kinz/kiface"

// Compile-time interface conformance assertions: the whole contract layer is
// verified at build time, so a signature drift fails CI immediately.
var (
	_ kiface.IServer           = (*Server)(nil)
	_ kiface.IConnection       = (*Connection)(nil)
	_ kiface.IMsgHandle        = (*MsgHandler)(nil)
	_ kiface.IConnManager      = (*ConnManager)(nil)
	_ kiface.IHeartbeatChecker = (*HeartBeatChecker)(nil)
	_ kiface.IClient           = (*Client)(nil)
	_ kiface.IDataPack         = (*DataPack)(nil)
	_ kiface.IMessage          = (*Message)(nil)
)
```

创建 `kinterceptor/interface_test.go`：

```go
package kinterceptor

import "kinz/kiface"

// Compile-time interface conformance assertions for the interceptor package.
var (
	_ kiface.IDecoder = (*FrameDecoder)(nil)
	_ kiface.IChain   = (*Chain)(nil)
)
```

- [ ] **Step 9: 全量构建 + 测试 + 覆盖率**

Run：

```powershell
go build ./...
go vet ./...
go test -race -cover ./...
```

Expected: build 绿；vet 无告警；所有测试 PASS（datapack 3 个 + 编译期断言）；输出中含 coverage 报告。

- [ ] **Step 10: 提交**

```bash
git add knet kinterceptor
git commit -m "feat(knet): add phase-2 stub implementations and compile-time interface assertions"
```

**Task 4 结束时的构建状态：全绿（build/vet/test -race）。**

---

## Task 6: 全量验证 + CLAUDE.md 同步

**Files:**
- Modify: `CLAUDE.md`（仅构建命令同步；完整重写在 P5）

- [ ] **Step 1: 同步 CLAUDE.md 命令**

将 CLAUDE.md 中 `go test ./znet/...`、`go test ./mmo_game_zinx/core/...` 等命令更新为当前可用的：

```bash
# Build
go build ./...

# Tests
go test ./knet/...
go test ./kinterceptor/...
```

（保留 go build 与 datapack 测试说明；删除 mmo 相关行。）

- [ ] **Step 2: 最终验证矩阵**

Run：

```powershell
go build ./...
go vet ./...
go test -race -cover ./...
go test ./knet/ -coverprofile=cover.out
go tool cover -func=cover.out
```

Expected:
- build/vet 全绿；
- `go test -race -cover ./...` 全部 PASS；
- `knet` 覆盖率：`datapack.go`/`message.go` ≥ 80%，total 记录为 P1 基线（桩代码拉低属预期）。

- [ ] **Step 3: 提交**

```bash
git add CLAUDE.md
git commit -m "docs: sync CLAUDE.md build commands after kinz rename"
```

---

## P1 退出标准核对

- [ ] `go build ./...` 绿
- [ ] `go vet ./...` 无告警
- [ ] `go test -race ./...` 全绿
- [ ] `kiface` 无僵尸方法（无 `Inotify/IFuncRequest/HandleFunc/Goto`）
- [ ] `knet` 无 `panic("implement me")`（桩返回 `ErrNotImplemented`）
- [ ] `klog` 测试全绿（级别过滤/JSON/With/InfoF 兼容，6 个用例）
- [ ] 所有接口有英文 doc comment
- [ ] 接口断言测试（knet/kinterceptor）通过
- [ ] 编解码层覆盖率 ≥ 80%，整体覆盖率报告已生成
- [ ] 每 Task 有独立提交，评审记录可追溯

---

## 后续阶段路线图（P2–P6，各阶段计划在上一阶段评审通过后单独编写）

- **P2 核心实现重写**（退出标准见设计文档 §7）：Server 生命周期（Run/Shutdown/Serve+信号）、Connection 状态机/缓冲写/IsAlive/缓冲池化、消息管线（bufio+Decoder 错误化+拦截器接入）、MsgHandler（RouterSlices/中间件/工作池优雅关闭/panic 恢复）、心跳接通、满连接拒绝、Request 完整实现、kconf（YAML）加载链；新增对应单元/集成测试。
- **P3 生产化**：klog 重写（slog+环形缓冲）、指标 kmetrics+Prometheus、TLS、Client 重写、哨兵错误落地；新增测试。
- **P4 MCP**：kmcp 包（协议/传输/工具/资源）、AttachMCP、鉴权回调；新增测试。
- **P5 文档与 AI 友好**：CLAUDE.md 重写、docs/ 全套、examples/（ping/chatroom/auth-middleware/mcp-stdio）、英文注释补齐。
- **P6 测试补强 + 发布**：模糊/基准测试、覆盖率门禁（核心 ≥ 70%）、CHANGELOG、v1.0.0。

> P7 `kinzctl`（发布后可选）见设计文档 §7，本次不实现。
