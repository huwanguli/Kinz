# Zinx 框架生产化重构设计

日期：2026-08-14
状态：草案（待评审）
模块：zinx（Go 1.25）

## 1. 背景与目标

Zinx 是一个参考原版 Zinx 实现的轻量级 TCP 服务器框架，当前处于"教学/课设"水平：
- 接口层（ziface）照抄了新版本 Zinx 的接口，实现层（znet）未跟上，**当前 `go build ./...` 直接失败**（接口漂移、拼写错误、缺失方法、`panic("implement me")`）。
- 心跳、拦截器链、Decoder、RouterSlices 等能力只有半成品，未接入主链路。
- 缺少生产环境必需的能力：优雅停机、满连接有反馈的拒绝、结构化日志、错误处理、指标、TLS、测试与 CI。

**目标**：将 Zinx 重构为一个可以上生产环境的 TCP 服务器框架，同时做到"AI 开发友好"（文档体系 + 内置 MCP Server 暴露运行时给 AI 工具）。

**已确认的决策**（用户拍板）：
1. 允许大改 API，目标是真正可上生产。
2. MCP 指"框架内置 MCP Server"：把运行中的服务器状态（连接、指标、配置）通过 MCP 暴露给 AI 工具，支持监控与操控。
3. 本次交付为详细重构计划文档，用户确认后再分阶段实施。

## 2. 现状问题清单（体检结果）

### 2.1 编译问题（阻断）
- `znet/server.go:39` 引用不存在的 `ziface.IDataPackd`（应为 `IDataPack`）。
- `IMsgHandle` 接口要求 `AddInterceptor/Execute/SetHeadInterceptor`，`MsgHandler` 未实现；且 `DoMsgHandler` 已从接口删除但 `connection.go:120` 仍调用。
- `IRequest` 要求 15+ 方法（`Abort/Call/GetMessage/BindRouter/RouterSlicesNext/Copy/Set/Get/...`），`Request` 只实现 3 个。
- `IMessage` 要求 `GetRawData`，`Message` 没有。
- `IConnection` 要求 `IsAlive/SetHeartBeat/LocalAddr` 等，`Connection` 没有。
- `IConnManager` 要求 `Get2/GetAllConnID/GetAllConnIdStr/Range/Range2`，`ConnManager` 没有。
- `IClient` 要求 `AddInterceptor/StartHeartBeat/StartHeartBeatWithOption/GetLengthField/SetDecoder/SetUrl/GetUrl` 等，`Client` 没有；且 `Client.Restart()` 的 dial 逻辑被注释掉，**根本不会连接**。
- `HeartBeatChecker` 调用的 `conn.IsAlive()`、`conn.SetHeartBeat()` 不存在。

### 2.2 功能缺口
- `Server` 中 15 个方法为 `panic("implement me")`：`AddRouterSlices/Group/Use/GetOnConnStart/GetOnConnStop/GetPacket/GetMsgHandler/SetPacket/StartHeartBeat/SetHeartBeatWithOption/GetHeartBeat/GetLengthField/SetDecoder/AddInterceptor/ServerName`。
- 解码器（`IDecoder`）与拦截器链（`IInterceptor`）**未接入 Connection 读取链路**，`StartReader` 硬编码走 `DataPack` 的 `io.ReadFull` 逐包读取，无粘包半包状态。
- 心跳检测器实现了但 Server 无法启动它；Connection 无存活跟踪。
- 满连接时只是静默 `conn.Close()`（代码留 TODO"给用户响应错误信息"）。

### 2.3 生产化缺陷
- **配置**：`utils.GlobalObject` 全局单例；`init()` 在 `conf/zinx.json` 缺失时 panic（该文件在 .gitignore 中，仓库内不存在，demo 一启动即崩）。
- **生命周期**：`Serve()` 用 `select{}` 死等；`Stop()` 无信号处理、无排空逻辑；`Server.Start()` 的错误只打印不返回。
- **连接并发安全**：`Stop()` 无锁（`isClosed` 竞态）、可能 double-close channel；`msgChan` 无缓冲——Writer 退出后 `SendMsg` 永久阻塞；`MaxConn` 检查与 `ConnManager.Add` 之间存在竞态（accept 循环与 Add 不同步）。
- **错误处理**：`FrameDecoder` 内部用 `panic` 表达协议错误；`Connection.Stop()` 里 `panic(err)`。
- **日志**：`fmt.Printf` 与 zlog 混用；zlog 无级别开关、无文件输出、硬编码 ANSI 颜色、`SetLevel` 实现有缺陷（赋值 `logger.SetOutput` 后条件判断逻辑颠倒）。
- **可观测性**：无任何指标。
- **测试**：仅 `datapack_test.go` 与 `aoi_test.go`；核心链路零覆盖。
- **僵尸代码**：`ziface.Inotify`、`IFuncRequest`、`HandleFunc`、`cID` 未用字段、`REPRO/TODEL` 等标记注释。

## 3. 方案对比

| 方案 | 做法 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| A 渐进修补 | 只修编译 + 补 panic 方法 | 快、改动小 | 根子问题（全局单例、fmt 日志、死等、panic）全保留，达不到生产 | 不采纳 |
| **B 骨架保留 + 全面重写（推荐）** | 保留 ziface/znet 包结构与命名习惯，接口与实现按生产标准重写，新增 zconf/zmcp | 架构清晰、成本可控（核心 ~2500 行）、demo 可迁移、API 可按需演进 | API 大改需要同步更新 demo 与文档 | **采纳** |
| C 推倒重来 v2 | 新模块新架构（如 netpoll） | 最彻底 | 工作量大、丢弃现有积累；goroutine-per-conn 本就是主流正确选择 | 不采纳 |

**采用方案 B。**

## 4. 目标架构

```
┌─────────────────────────── 应用层（业务代码）───────────────────────────┐
│   IRouter（经典三段式） / IRouterSlices（函数式+中间件）                    │
└──────────────────────────────────────────────────────────────────────┘
┌─────────────────────────── 框架层（zinx）──────────────────────────────┐
│  Server（生命周期 Run/Shutdown，Option 配置）                            │
│  ├─ Listener（TCP / TLS，Accept → 满连接拒绝 → NewConnection）           │
│  ├─ Connection（读协程 + 写协程，原子状态机，IsAlive）                    │
│  │    ├─ bufio.Reader → IDecoder(拆粘包/半包) → IInterceptor链            │
│  │    └─ msgChan(缓冲+超时) → Writer → socket                           │
│  ├─ MsgHandler（RouterSlices/经典路由，Worker 池，panic 恢复）            │
│  ├─ HeartBeatChecker（Server 配置 → 每连接 Clone，超时回调）              │
│  ├─ ConnManager（原子计数，满连接拒绝钩子）                               │
│  └─ zmetrics（连接/消息/错误/字节/心跳丢失/队列深度）                       │
├─ zlog（ILogger 接口 + slog 实现，结构化、可配）                            │
├─ zconf（Config + 默认值/JSON/env/Option 加载链）                          │
└─ zmcp（MCP Server：stdio/TCP，暴露 Runtime 给 AI 工具）                   │
└──────────────────────────────────────────────────────────────────────┘
```

数据流（读）：`socket → bufio.Reader → IDecoder（帧重组）→ IMessage → IInterceptor 链（鉴权/解密/压缩等）→ IRequest → MsgHandler（按 MsgID 分发）→ Worker 池或直启 goroutine → 业务 Router`。
数据流（写）：`业务 → conn.SendMsg（封包）→ msgChan → Writer goroutine → socket`。

## 5. 详细设计

### 5.1 配置体系（新增包 `zconf`，替代 `utils.GlobalObject`）

- `Config` 结构体字段：`Name, Host, Port, MaxConn, MaxPacketSize, WorkerPoolSize, MaxWorkerTaskLen, HeartbeatInterval, HeartbeatTimeout, WriteQueueSize, WriteTimeout, ReadIdleTimeout, TLSConfig, Logger` 等。
- 加载链（优先级从低到高）：内置默认值 → `conf/zinx.json`（**缺失不 panic**，仅跳过）→ 环境变量（`ZINX_*`）→ 代码 Option 覆盖。
- `znet.NewServer(opts ...Option)` 函数式选项：`WithConfig(*zconf.Config)`、`WithTLS(*tls.Config)`、`WithMaxConn(n)` 等。
- 删除 `utils` 包中对全局单例的依赖；`utils` 包整体移除，其职责并入 `zconf`。
- 旧 `conf/zinx.json` 格式继续被支持（作为可选加载源），保证配置平滑迁移。

### 5.2 服务器生命周期

- `Server.Run(ctx) error`：解析监听地址（支持 `:port` 简写）→ 启动 Worker 池 → 开始 Accept 循环；返回错误而非打印。
- `Server.Shutdown(ctx) error`（优雅停机）：
  1. 停止 Accept（关闭 listener）；
  2. 通知所有连接进入排空（停止心跳、等待在途消息写完），带 `ctx` 超时；
  3. 关闭 Worker 池（等待队列排空或超时）；
  4. 清理资源，返回。
- `Server.Serve(ctx)`：`Run` + 阻塞等待 ctx 取消 + `Shutdown` 的组合封装，附带 `signal.NotifyContext` 示例（SIGINT/SIGTERM）。
- 删除 `select{}` 死等模式。
- 连接钩子（OnConnStart/OnConnStop）保留，但语义明确为"在连接创建/销毁时同步调用，panic 会被框架 recover 并记日志"。

### 5.3 连接模型（重写 `Connection`）

- **状态机**：`created → running → closing → closed`，用 `atomic.Uint32` + CAS 保证幂等 `Stop()`；channel 只关闭一次（`sync.Once`）。
- **写路径**：`msgChan` 改为可配置缓冲（`WriteQueueSize`）；`SendMsg` 用
  `select { case msgChan <- data: ...; case <-done: ...; case <-time.After(WriteTimeout): ... }` 防止永久阻塞；Writer 退出后 SendMsg 返回 `ErrConnClosed`。
- **读路径**：`bufio.Reader` + `IDecoder` 处理粘包/半包（替换逐包 `io.ReadFull`）；解码出的完整帧送入拦截器链。
- **存活跟踪**：`lastActivity` 原子时间戳，任何收到消息（不只心跳）都刷新；`IsAlive(timeout)` 基于此判断。
- **读写超时**：可选 `ReadIdleTimeout`（配合心跳做兜底）；`SetWriteDeadline` 防对端不读导致的写阻塞。
- **错误处理**：读写 goroutine 内 `recover()`，记录日志并触发连接关闭，不让单连接错误打崩进程。
- **ConnID**：`uint64`（与 `IConnManager` 接口对齐）。
- 属性（SetProperty/GetProperty/RemoveProperty）保留，继续由 RWMutex 保护。

### 5.4 消息管线（打通解码器 + 拦截器）

- 默认解码器：`zinterceptor.FrameDecoder`（LengthField 通用帧解码，支持大小端、1/2/3/4/8 字节长度域），**其内部 `panic` 全部改为返回 error**；协议错误 → 关闭该连接 + 记日志 + 计数指标。
- 拦截器链：`IInterceptor` 责任链在 `MsgHandler.Execute(request)` 中执行，位于路由分发之前；`SetHeadInterceptor` 允许插入链头。
- 工作池：保留"按 ConnID 取模分配 Worker 保证同连接有序"，增加：
  - 池大小与队列长度可配置（来自 zconf）；
  - 优雅关闭（ctx 取消后排空队列再退出）；
  - 队列打满时的背压策略：阻塞发送（保持有序）并在指标上计数 `queue_full`（v1 不做丢弃）。
- 无池模式（WorkerPoolSize=0）：每个消息一个 goroutine，同样受 panic 恢复保护。

### 5.5 路由体系（补齐 RouterSlices）

- 完整实现 `IRouterSlices`：`Use`（全局中间件）、`Group(start, end, ...)`（区间分组中间件）、`AddHandler(msgID, ...)`、`GetHandlers`。
- `Request` 完整实现 `IRequest`：`Abort`（终止后续处理器）、`Call/RouterSlicesNext`（链式推进）、`Set/Get`（请求级上下文）、`Copy`、`GetMessage/GetResponse/SetResponse`、`BindRouter/BindRouterSlices`。
- 经典 `IRouter`（PreHandle/Handle/PostHandle）保留，`BaseRouter` 保留。
- 注册期校验：msgID 重复注册返回 error（不再 panic）。

### 5.6 心跳保活（接通主链路）

- Server 级配置：`HeartbeatInterval`、超时判定（`HeartbeatTimeout`）、消息 MsgID（默认 99999）。
- 接线：`OnConnStart` 钩子内部（框架自动）为每个新连接 `Clone` 一份 `HeartBeatChecker` 并 `BindConn`；无需业务代码干预，也可通过 Option 关闭。
- `HeartBeatChecker.check()`：若 `!conn.IsAlive(timeout)` → `OnRemoteNotAlive`（默认：优雅断开）；否则按 `beatFunc` 发送心跳消息（默认发送心跳帧）。
- 服务端同时监听客户端心跳帧（默认路由 `HeartBeatDefaultRouter`，收到即刷新存活）。
- 客户端侧（`Client`）同样内置心跳发送。
- 修复原实现的明显 bug：`SetHeartBeatMsgFunc`/`SetHeartbeatFunc` 中 `if msgFunc == nil` 判断写反（nil 时才赋值），改为 `!= nil`。

### 5.7 满连接拒绝（有反馈的断开）

- 配置 `MaxConn`；`ConnManager` 维护原子计数（或加锁检查 + Add 原子完成），消除 accept 与 Add 之间的竞态。
- 超限时行为（可配置）：
  1. 默认：向客户端发送预定义"服务器已满"错误消息（`ErrServerFullMsgID`，内容可配），随后优雅关闭；
  2. `OnConnRejected` 钩子（记录来源、指标）；
  3. 计数指标 `conns_rejected`。
- 禁止默默 Close（现状）与裸写响应（半包风险）的做法。

### 5.8 日志（重写 `zlog`）

- `ILogger` 接口扩展：`Debug/Info/Warn/Error` 及 `With(fields)`（结构化上下文），保留 `InfoF/ErrorF` 兼容形式。
- 默认实现基于标准库 `log/slog`：级别可配（DEBUG/INFO/WARN/ERROR）、输出可配（stdout/文件/io.Writer）、格式可选（text/JSON）、**无硬编码颜色**。
- 提供 `zlog.SetDefault` 全局默认 + `Option` 注入 `Server` 级 logger。
- 提供可选的**环形缓冲日志后端**（带容量上限），供 MCP `get_logs` 工具读取最近日志（见 5.13）。
- 框架内部所有 `fmt.Printf` 日志迁移到 `zlog`（保留 Accept 等关键事件的结构化字段：remote addr、connID、msgID 等）。

### 5.9 错误处理与 panic 恢复

- 哨兵错误：`ErrServerClosed`、`ErrConnClosed`、`ErrTooLargePacket`、`ErrServerFull`、`ErrProtocol`、`ErrTimeout`。
- `MsgHandler` 执行 Router 时 `defer recover()`：记录堆栈、计数 `handler_panics`、不中断其他消息。
- 读写 goroutine 与 Worker 同规则；`FrameDecoder` 不再 panic。
- 所有 `panic("implement me")` 在 P1/P2 消灭。

### 5.10 指标（轻量可观测性）

- 框架内置指标注册表（`zmetrics`，原子计数器/直方图，零外部依赖）：
  - `conns_total`（累计创建）、`conns_active`（当前活跃）、`conns_closed`、`conns_rejected`；
  - `msgs_received`、`msgs_sent`、`msgs_dropped`、`bytes_in`、`bytes_out`；
  - `handler_panics`、`heartbeat_missed`、`queue_full`、`worker_busy`；
  - 直方图：`msg_handle_duration`、`conn_lifetime`。
- 暴露方式：MCP 工具/资源读取（P4）+ 可选 Prometheus 文本端点（`/metrics`，独立小 HTTP listener，P3）。
- 不引入重量级监控依赖；如需 OpenTelemetry，通过接口后续扩展。

### 5.11 TLS（P3）

- `Option` 注入 `*tls.Config`；Server 侧 `tls.Listen` 包装 listener；Client 侧支持 TLS 拨号。
- 仅作为可选能力，默认纯 TCP 不变。

### 5.12 Client 重写（P3）

- 修复核心缺陷：实现真正 `dial`（`net.Dialer.DialContext`，连接超时可配）。
- 自动重连：指数退避 + 抖动，`Restart/Stop` 语义明确（Stop 后不再自动重连）。
- 与 Server 同构的消息管线：bufio+Decoder、拦截器、路由、心跳发送、`SendMsg` 非阻塞写。
- 连接状态回调（OnConnStart/OnConnStop）语义与 Server 对齐。

### 5.13 MCP Server（新增包 `zmcp`，P4）

**定位**：把运行中的 Zinx 服务器暴露给 AI 工具（Claude Desktop、IDE、任意 MCP 客户端），实现"AI 开发友好"的运行时闭环。

- **协议**：MCP（Model Context Protocol），JSON-RPC 2.0 子集；实现 `initialize` 握手、`tools/list`、`tools/call`、`resources/list`、`resources/read`。
- **传输**：v1 支持 stdio（最简单、Claude Desktop 原生支持）与 TCP 两种 transport，通过 `Transport` 接口抽象，后续可平滑接入官方/第三方 SDK（如 `mark3labs/mcp-go`），**v1 零新增外部依赖**（手写 JSON-RPC 编解码，量小可控）。
- **暴露的工具（Tools）**：
  - `server_info`：名称、版本、监听地址、启动时间；
  - `list_connections`：在线连接列表（ID/远端/存活时间/收发字节）；
  - `get_connection`：单连接详情；
  - `send_to_connection`：向指定连接发送消息；
  - `broadcast`：广播消息；
  - `close_connection`：优雅关闭指定连接；
  - `get_metrics`：全部指标；
  - `get_config`：当前生效配置；
  - `get_logs`：最近日志（环形缓冲）；
  - `shutdown_server`：触发优雅停机。
- **资源（Resources）**：`connections://`、`metrics://`、`config://`、`logs://`。
- **接入方式**：
  - `srv.AttachMCP(zmcp.WithTCP(addr))`：开启 TCP MCP 端点；
  - `zmcp.NewServer(srv).RunStdio()`：stdio 模式（给 Claude Desktop 配置文件指向可执行文件）。
- 权限模型：工具调用默认放行（开发/内网运维场景），预留鉴权回调接口（`WithAuth(func(call) error)`）。
- **AI 友好文档**：`docs/mcp.md` 说明如何在 Claude Desktop / IDE 中配置接入、可用的工具与资源清单、示例对话。

### 5.14 文档与 AI 友好（P5）

- **CLAUDE.md 重写**：架构总览、构建/测试命令、代码约定（接口先行、错误处理、命名）、目录导航、常见坑。
- **`docs/` 目录**：
  - `architecture.md`：包结构、数据流、生命周期图；
  - `protocol.md`：TLV 线协议与 LengthField 帧格式、字节序、示例报文；
  - `configuration.md`：zconf 全字段说明 + JSON/env 示例；
  - `getting-started.md`：快速开始（服务端/客户端最小示例）；
  - `examples/`：ping、聊天室、鉴权中间件、MCP 接入示例；
  - `mcp.md`：MCP Server 配置与使用；
  - `testing.md`：测试命令、模糊测试、基准测试；
  - `faq.md` 与 `production-checklist.md`（部署/压测/调参）。
- **代码注释规范**：所有导出符号补齐英文 doc comment（AI 与国际化友好）；中文保留在 README 与设计文档。
- `INTERVIEW_GUIDE.md` 保留并在 P6 按新架构同步更新（属用户个人资产）。
- MMO demo 作为"真实业务参考"迁移到新 API，继续充当架构验证与教学样例。

### 5.15 测试与 CI（P0 骨架 + P6 补强）

- **单元测试**：datapack（保留）、framedecoder（全分支）、connmanager、heartbeat（超时/存活/自定义回调）、router & slices（中间件顺序/Abort/Group）、request（上下文/Copy）、msgHandler（分发/panic 恢复）、connection 生命周期（`net.Pipe` 驱动读写）。
- **集成测试**（真实 TCP loopback）：echo 往返、粘包半包重组、心跳超时断开、满连接拒绝（收到错误消息）、优雅停机（在途消息写完）、TLS 握手、Client 重连。
- **模糊测试**：`FrameDecoder.Decode`（`go test -fuzz`）。
- **基准测试**：Pack/Unpack、单连接吞吐、多连接并发。
- **CI**（GitHub Actions）：`go vet` + `go test -race ./...` + `golangci-lint` + 覆盖率门禁（核心包 ≥ 70%）+ 构建产物检查。本地配套 Makefile/Taskfile 命令。

## 6. 分阶段实施计划（每阶段独立验收，顺序执行）

### Phase 0 — 基线锁定（P0）
- 内容：添加 CI 骨架（build + vet 检查，允许标红）、记录当前构建失败清单、整理 todo。
- 退出标准：CI 配置文件落地；构建失败清单与本文档 2.1 一致。

### Phase 1 — 接口重定义（P1）
- 内容：按新设计重写 `ziface` 全部接口（精简删除僵尸方法：`Inotify/IFuncRequest/HandleFunc` 等；明确 `IServer/IConnection/IRequest/IMessage/IMsgHandle/IConnManager/IClient` 语义与注释）；新增 `zconf`；删除 `utils` 依赖；为旧实现提供最小占位以保证 `go build ./...` 通过。
- 退出标准：`go build ./...` 绿；`ziface` 无僵尸方法；`znet` 中不再有 `panic("implement me")`（未实现的方法以返回零值/`ErrNotImplemented` 的占位实现过渡，P2 完成真实现）；所有接口有英文注释。

### Phase 2 — 核心实现重写（P2）
- 内容：`Server`（Run/Shutdown/Serve+信号）、`Connection`（状态机/缓冲写/存活/错误处理）、消息管线（bufio+Decoder+拦截器接入）、`MsgHandler`（RouterSlices/中间件/工作池优雅关闭/panic 恢复）、心跳接通、满连接拒绝、`Request` 完整实现。
- 退出标准：ping demo 用新 API 跑通；心跳超时断开验证通过；满连接返回错误消息验证通过；中间件/Group/Abort 行为验证通过；SIGTERM 优雅停机（在途消息写完）验证通过；`go test ./...` 核心用例绿。

### Phase 3 — 生产化（P3）
- 内容：zlog 重写（slog）、全部日志迁移、哨兵错误、指标注册表 + Prometheus 端点、TLS、Client 完整重写（重连/心跳/超时）。
- 退出标准：日志结构化可查；panic 处理器不炸进程；指标可读；TLS 握手验证通过；Client 断线重连验证通过。

### Phase 4 — MCP Server（P4）
- 内容：`zmcp` 包（协议、stdio/TCP transport、工具与资源）、`AttachMCP` 接入、鉴权回调、`docs/mcp.md`。
- 退出标准：任意 MCP 客户端（或手写最小客户端）能完成 initialize 握手、列出工具、读取连接/指标、向连接发消息、广播、关连接、触发停机。

### Phase 5 — 文档与 AI 友好（P5）
- 内容：CLAUDE.md 重写、`docs/` 全套、导出符号英文注释补齐、示例应用（ping/聊天室/鉴权/MCP）。
- 退出标准：文档与代码一致；示例可运行；注释规范抽查通过。

### Phase 6 — 测试补强 + demo 迁移 + 发布（P6）
- 内容：单元/集成/模糊/基准测试补全、MMO demo 迁移新 API、`INTERVIEW_GUIDE.md` 更新、CI 全绿、CHANGELOG。
- 退出标准：`go test -race ./...` 全绿；核心包覆盖率 ≥ 70%；fuzz 与 bench 可运行并有基线；MMO demo 正常游玩；CI 全绿。

## 7. 兼容性与迁移

- 保留默认 TLV（小端）线协议不变，保证老客户端可通信；`ziface` 中已声明的 `ZinxDataPack`（大端）作为可选。
- API 破坏点明确列出：`NewServer` → `NewServer(opts...)`；`Serve()` → `Serve(ctx)`/`Run(ctx)`；`utils.GlobalObject` → `zconf`；`SendMsg` 返回错误语义不变。
- demo 与 MMO 在 P2/P6 分别迁移，确保每个阶段都有可运行的示例。
- 迁移顺序：框架核心 → 官方示例 → MMO demo → 面试文档。

## 8. 风险与取舍

| 风险 | 应对 |
|------|------|
| 大改 API 导致现有 demo/文档失效 | 分阶段迁移（P2 迁移 demo，P6 迁移 MMO），每阶段可运行 |
| MCP 协议/SDK 生态变化 | transport 抽象隔离，v1 零依赖手写实现，可换 SDK |
| 重写范围大、回归风险 | 接口先行（P1）+ 测试先行（P0 骨架，P2 起逐功能补测试）+ 每阶段独立验收 |
| Worker 队列打满 | v1 阻塞背压 + `queue_full` 指标；预留丢弃/淘汰策略扩展点 |
| 心跳误杀慢业务 | 存活判定基于"任意消息"而非仅心跳帧；超时参数可配；心跳只作兜底，另有 ReadIdleTimeout 可选 |

## 9. 验收标准（Definition of Done）

1. `go build ./...`、`go vet ./...`、`go test -race ./...` 全绿。
2. 核心包单元/集成测试覆盖率 ≥ 70%，模糊与基准测试可用。
3. 心跳、满连接拒绝、优雅停机、TLS、Client 重连均有集成测试覆盖。
4. MCP Server 可被标准 MCP 客户端接入并完成监控/操控演示。
5. `docs/` 与 CLAUDE.md 完整且与代码一致；导出符号有英文注释。
6. demo 与 MMO demo 迁移到新 API 并可运行。
7. 框架内无 `panic("implement me")`、无裸 `panic` 表达协议错误、无 `fmt.Printf` 日志残留。
