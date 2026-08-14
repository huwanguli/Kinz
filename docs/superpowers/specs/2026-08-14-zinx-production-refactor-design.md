# Kinz 框架生产化重构设计

日期：2026-08-14
状态：v2 草案（待评审）
模块：kinz（Go 1.25）

## 1. 背景与目标

现有代码（Zinx）是一个参考原版 Zinx 实现的轻量级 TCP 服务器框架，处于"教学/课设"水平：
- 接口层（ziface）照抄了新版本 Zinx 的接口，实现层（znet）未跟上，**当前 `go build ./...` 直接失败**（接口漂移、拼写错误、缺失方法、`panic("implement me")`）。
- 心跳、拦截器链、Decoder、RouterSlices 等能力只有半成品，未接入主链路。
- 缺少生产环境必需的能力：优雅停机、满连接有反馈的拒绝、结构化日志、错误处理、指标、TLS、测试与 CI。

**目标**：将框架重构为 **Kinz**——一个可以上生产环境的 TCP 服务器框架，同时做到"AI 开发友好"（文档体系 + 内置 MCP Server 暴露运行时给 AI 工具）。

**已确认的决策**（用户拍板）：
1. 允许大改 API，目标是真正可上生产。
2. MCP 指"框架内置 MCP Server"：把运行中的服务器状态（连接、指标、配置）暴露给 AI 工具，支持监控与操控。
3. 本次交付为详细重构计划文档，用户确认后再分阶段实施。
4. **设计哲学：约定为主 + 明确扩展点**（见 §2）。
5. **全面改名**：module 为 `kinz`，包为 `kiface/knet/klog/kconf/kmcp/kmetrics/kinterceptor`；`utils` 包移除（职责并入 `kconf`）。
6. **读协程缓冲用 `sync.Pool` 池化**（见 §6.3）。
7. **配置文件用 YAML**（`gopkg.in/yaml.v3`，`conf/kinz.yaml`），不再用 JSON。
8. **mmo 与 demo 不重构，直接删除归档**（git 历史可回溯）；框架示例（examples/）在 P5 以新 API 新建。
9. **每一步交付新测试**：每个阶段的功能与其测试同批落地、同批通过。

## 2. 设计哲学：约定为主 + 明确扩展点

**核心思想：框架约定"管线的形状"，业务决定"管线的内容"。**

### 2.1 约定的内容（默认路径，零配置可用）

| 维度 | 约定 |
|------|------|
| 管线形状 | 固定：`accept → 解码 → 拦截链 → 路由分发 → 业务`，形状不可改 |
| 安全默认 | 心跳默认开启、满连接拒绝默认有反馈、处理器 panic 默认恢复、优雅停机默认支持 |
| 路由 | 按 MsgID 注册，中间件机制内置 |
| 日志/指标 | 内置 slog 实现 + 指标注册表，开箱即用 |
| 配置 | 内置默认值直接可跑，`conf/kinz.yaml` 覆盖常用项，Option 编程式覆盖 |

### 2.2 扩展点（seam，允许替换/插拔）

| 扩展点 | 接口 | 默认实现 |
|--------|------|----------|
| 编解码 | `IDecoder` / `IDataPack` | LengthField 帧解码器 + TLV（小端） |
| 中间件 | `IInterceptor`（责任链） | 空链（可插入鉴权/加解密/限流） |
| 路由 | `IRouter` / `IRouterSlices` | 经典三段式 + 函数式中间件链 |
| 日志 | `ILogger` | slog 封装 |
| 指标 | `IMetrics` | 原子计数/直方图注册表 |

### 2.3 为什么这样选

- **生产就绪不是事后补**：心跳、满连接拒绝、panic 恢复、优雅停机由框架默认决策，业务代码无需记得开启。
- **AI 友好**：默认路径清晰、文档短，AI 生成的代码更可能正确；扩展点有明确接口与默认实现，AI 按 seam 填充即可。
- **避免 Zinx 覆辙**：原 Zinx 接口驱动但实现跟不上（15 个 `panic("implement me")`）。Kinz 先约定默认路径（全部有实现），再提供 seam，杜绝"接口先铺、实现后补"。
- **TCP 场景差异**：游戏/IoT/聊天/RPC 的差异集中在编解码与中间件，正好落在 seam 上；管线形状的约定不构成束缚。

## 3. 现状问题清单（被重构代码的体检结果）

### 3.1 编译问题（阻断）
- `znet/server.go:39` 引用不存在的 `ziface.IDataPackd`（应为 `IDataPack`）。
- `IMsgHandle` 接口要求 `AddInterceptor/Execute/SetHeadInterceptor`，`MsgHandler` 未实现；且 `DoMsgHandler` 已从接口删除但 `connection.go:120` 仍调用。
- `IRequest` 要求 15+ 方法（`Abort/Call/GetMessage/BindRouter/RouterSlicesNext/Copy/Set/Get/...`），`Request` 只实现 3 个。
- `IMessage` 要求 `GetRawData`，`Message` 没有。
- `IConnection` 要求 `IsAlive/SetHeartBeat/LocalAddr` 等，`Connection` 没有。
- `IConnManager` 要求 `Get2/GetAllConnID/GetAllConnIdStr/Range/Range2`，`ConnManager` 没有。
- `IClient` 要求 `AddInterceptor/StartHeartBeat/StartHeartBeatWithOption/GetLengthField/SetDecoder/SetUrl/GetUrl` 等，`Client` 没有；且 `Client.Restart()` 的 dial 逻辑被注释掉，**根本不会连接**。
- `HeartBeatChecker` 调用的 `conn.IsAlive()`、`conn.SetHeartBeat()` 不存在。

### 3.2 功能缺口
- `Server` 中 15 个方法为 `panic("implement me")`：`AddRouterSlices/Group/Use/GetOnConnStart/GetOnConnStop/GetPacket/GetMsgHandler/SetPacket/StartHeartBeat/SetHeartBeatWithOption/GetHeartBeat/GetLengthField/SetDecoder/AddInterceptor/ServerName`。
- 解码器（`IDecoder`）与拦截器链（`IInterceptor`）**未接入 Connection 读取链路**，`StartReader` 硬编码走 `DataPack` 的 `io.ReadFull` 逐包读取，无粘包半包状态。
- 心跳检测器实现了但 Server 无法启动它；Connection 无存活跟踪。
- 满连接时只是静默 `conn.Close()`（代码留 TODO"给用户响应错误信息"）。

### 3.3 生产化缺陷
- **配置**：`utils.GlobalObject` 全局单例；`init()` 在 `conf/zinx.json` 缺失时 panic（该文件在 .gitignore 中，仓库内不存在，demo 一启动即崩）。
- **生命周期**：`Serve()` 用 `select{}` 死等；`Stop()` 无信号处理、无排空逻辑；`Server.Start()` 的错误只打印不返回。
- **连接并发安全**：`Stop()` 无锁（`isClosed` 竞态）、可能 double-close channel；`msgChan` 无缓冲——Writer 退出后 `SendMsg` 永久阻塞；`MaxConn` 检查与 `ConnManager.Add` 之间存在竞态。
- **错误处理**：`FrameDecoder` 内部用 `panic` 表达协议错误；`Connection.Stop()` 里 `panic(err)`。
- **日志**：`fmt.Printf` 与 zlog 混用；zlog 无级别开关、无文件输出、硬编码 ANSI 颜色、`SetLevel` 实现有缺陷。
- **可观测性**：无任何指标。
- **测试**：仅 `datapack_test.go` 与 `aoi_test.go`；核心链路零覆盖。
- **僵尸代码**：`ziface.Inotify`、`IFuncRequest`、`HandleFunc`、`cID` 未用字段、`REPRO/TODEL` 等标记注释。

## 4. 方案对比

| 方案 | 做法 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| A 渐进修补 | 只修编译 + 补 panic 方法 | 快、改动小 | 根子问题（全局单例、fmt 日志、死等、panic）全保留，达不到生产 | 不采纳 |
| **B 骨架保留 + 全面重写（推荐）** | 保留"接口层/实现层"的包结构习惯，按新哲学重写接口与实现，新增 kconf/kmcp | 架构清晰、成本可控、可逐步验证 | API 大改（已允许） | **采纳** |
| C 推倒重来 v2 | 全新模块新架构（如 netpoll） | 最彻底 | 工作量大、丢弃积累；goroutine-per-conn 本就是主流正确选择 | 不采纳 |

**采用方案 B，包结构：**

```
kinz/
├── kiface/        接口层（约定 + seam 的契约）
├── knet/          实现层（Server/Connection/MsgHandler/HeartBeat/ConnManager/Client）
├── kinterceptor/  FrameDecoder（LengthField 帧解码，错误返回而非 panic）+ Chain
├── klog/          ILogger 接口 + slog 实现（含环形缓冲日志后端）
├── kconf/         Config + 默认值/YAML/env/Option 加载链
├── kmetrics/      指标注册表（原子计数/直方图）
├── kmcp/          MCP Server（stdio/TCP，暴露运行时给 AI 工具）
├── examples/      新 API 示例（ping/聊天室/鉴权中间件/MCP 接入）
├── conf/          kinz.yaml 示例
└── docs/          架构/协议/配置/入门/MCP/测试文档
```

## 5. 目标架构

```
┌─────────────────────────── 应用层（业务代码）───────────────────────────┐
│   IRouter（经典三段式） / IRouterSlices（函数式+中间件）                    │
└──────────────────────────────────────────────────────────────────────┘
┌─────────────────────────── 框架层（kinz）──────────────────────────────┐
│  Server（生命周期 Run/Shutdown，Option 配置）                            │
│  ├─ Listener（TCP / TLS，Accept → 满连接拒绝 → NewConnection）           │
│  ├─ Connection（读协程 + 写协程，原子状态机，IsAlive，缓冲池化）            │
│  │    ├─ bufio.Reader（sync.Pool 池化）→ IDecoder → IInterceptor链        │
│  │    └─ msgChan(缓冲+超时) → Writer → socket                           │
│  ├─ MsgHandler（RouterSlices/经典路由，Worker 池，panic 恢复）            │
│  ├─ HeartBeatChecker（Server 配置 → 每连接 Clone，超时回调）              │
│  ├─ ConnManager（原子计数，满连接拒绝钩子）                               │
│  └─ kmetrics（连接/消息/错误/字节/心跳丢失/队列深度）                       │
├─ klog（ILogger + slog，结构化、可配、环形缓冲）                            │
├─ kconf（Config + 默认值/YAML/env/Option 加载链）                          │
└─ kmcp（MCP Server：stdio/TCP，暴露 Runtime 给 AI 工具）                   │
└──────────────────────────────────────────────────────────────────────┘
```

数据流（读）：`socket → bufio.Reader（池化缓冲）→ IDecoder（帧重组，缓冲池化）→ IMessage → IInterceptor 链 → IRequest → MsgHandler（按 MsgID 分发）→ Worker 池或直启 goroutine → 业务 Router`。
数据流（写）：`业务 → conn.SendMsg（封包，写缓冲池化）→ msgChan → Writer goroutine → socket`。

## 6. 详细设计

### 6.1 配置体系（新包 `kconf`，YAML）

- `Config` 结构体字段：`Name, Host, Port, MaxConn, MaxPacketSize, WorkerPoolSize, MaxWorkerTaskLen, HeartbeatInterval, HeartbeatTimeout, WriteQueueSize, WriteTimeout, ReadIdleTimeout, TLSConfig, Logger, Metrics` 等，全部带 `yaml` tag。
- **加载链**（优先级从低到高）：内置默认值 → `conf/kinz.yaml`（**缺失不 panic**，仅跳过）→ 环境变量（`KINZ_*`）→ 代码 Option 覆盖。
- **YAML 解析**：`gopkg.in/yaml.v3`（唯一新增的外部依赖；框架核心其余零依赖）。
- `knet.NewServer(opts ...Option)` 函数式选项：`WithConfig(*kconf.Config)`、`WithTLS(*tls.Config)`、`WithMaxConn(n)` 等。
- 删除 `utils` 包（含 `GlobalObject` 全局单例）；`conf/kinz.yaml` 仅作可选覆盖源，仓库内提供示例文件。

### 6.2 服务器生命周期

- `Server.Run(ctx) error`：解析监听地址（支持 `:port` 简写）→ 启动 Worker 池 → 开始 Accept 循环；返回错误而非打印。
- `Server.Shutdown(ctx) error`（优雅停机）：
  1. 停止 Accept（关闭 listener）；
  2. 通知所有连接进入排空（停止心跳、等待在途消息写完），带 `ctx` 超时；
  3. 关闭 Worker 池（等待队列排空或超时）；
  4. 归还池化缓冲资源、清理指标，返回。
- `Server.Serve(ctx)`：`Run` + 阻塞等待 ctx 取消 + `Shutdown` 的组合封装，附带 `signal.NotifyContext` 示例（SIGINT/SIGTERM）。
- 删除 `select{}` 死等模式。
- 连接钩子（OnConnStart/OnConnStop）保留，语义明确：同步调用，panic 由框架 recover 并记日志。

### 6.3 连接模型（重写 `Connection`）

- **状态机**：`created → running → closing → closed`，用 `atomic.Uint32` + CAS 保证幂等 `Stop()`；channel 只关闭一次（`sync.Once`）。
- **写路径**：`msgChan` 改为可配置缓冲（`WriteQueueSize`）；`SendMsg` 用
  `select { case msgChan <- data: ...; case <-done: ...; case <-time.After(WriteTimeout): ... }` 防止永久阻塞；Writer 退出后 SendMsg 返回 `ErrConnClosed`。
- **读路径**：`bufio.Reader` + `IDecoder` 处理粘包/半包；解码出的完整帧送入拦截器链。
- **读缓冲池化（sync.Pool）**：
  - 新增独立小包 **`kpool`**（导出 `BufferPool`，便于单测）：按 **4K/16K/64K** 三个尺寸分类的 `sync.Pool`；`Get(size)` 返回不小于请求尺寸的缓冲，`Put(buf)` 归还（校验尺寸档位，首尾字节写哨兵以便误用检测）。
  - 应用点：① 每连接 `bufio.Reader` 的底层缓冲（连接创建时 Get、关闭时 Put）；② `FrameDecoder` 内部累积缓冲 `in`；③ 解码产物消息载荷 `data`；④ `Pack` 封包写缓冲。
  - 归还时机：统一在连接关闭路径归还，保证池不泄漏；被拦截器/业务持久持有的缓冲**不池化**（文档注明，避免误用）。
  - 收益：高并发下避免每连接 4K 起步的分配/GC 压力；基准测试验证（见 §6.15）。
- **存活跟踪**：`lastActivity` 原子时间戳，任何收到消息（不只心跳）都刷新；`IsAlive(timeout)` 基于此判断。
- **读写超时**：可选 `ReadIdleTimeout`（配合心跳做兜底）；`SetWriteDeadline` 防对端不读导致的写阻塞。
- **错误处理**：读写 goroutine 内 `recover()`，记录日志并触发连接关闭，不让单连接错误打崩进程。
- **ConnID**：`uint64`。
- 属性（SetProperty/GetProperty/RemoveProperty）保留，由 RWMutex 保护。

### 6.4 消息管线（打通解码器 + 拦截器）

- 默认解码器：`kinterceptor.FrameDecoder`（LengthField 通用帧解码，支持大小端、1/2/3/4/8 字节长度域），**内部 `panic` 全部改为返回 error**；协议错误 → 关闭该连接 + 记日志 + 计数指标。
- 拦截器链：`IInterceptor` 责任链在 `MsgHandler.Execute(request)` 中执行，位于路由分发之前；`SetHeadInterceptor` 允许插入链头。
- 工作池：保留"按 ConnID 取模分配 Worker 保证同连接有序"，增加：
  - 池大小与队列长度可配置（来自 kconf）；
  - 优雅关闭（ctx 取消后排空队列再退出）；
  - 队列打满时的背压策略：阻塞发送（保持有序）并在指标上计数 `queue_full`（v1 不做丢弃）。
- 无池模式（WorkerPoolSize=0）：每个消息一个 goroutine，同样受 panic 恢复保护。

### 6.5 路由体系（补齐 RouterSlices）

- 完整实现 `IRouterSlices`：`Use`（全局中间件）、`Group(start, end, ...)`（区间分组中间件）、`AddHandler(msgID, ...)`、`GetHandlers`。
- `Request` 完整实现 `IRequest`：`Abort`（终止后续处理器）、`Call/RouterSlicesNext`（链式推进）、`Set/Get`（请求级上下文）、`Copy`、`GetMessage/GetResponse/SetResponse`、`BindRouter/BindRouterSlices`。
- 经典 `IRouter`（PreHandle/Handle/PostHandle）保留，`BaseRouter` 保留。
- 注册期校验：msgID 重复注册返回 error（不再 panic）。

### 6.6 心跳保活（接通主链路）

- Server 级配置：`HeartbeatInterval`、超时判定（`HeartbeatTimeout`）、消息 MsgID（默认 99999）。
- 接线：框架在连接建立时自动为每个新连接 `Clone` 一份 `HeartBeatChecker` 并 `BindConn`；无需业务代码干预，可通过 Option 关闭。
- `HeartBeatChecker.check()`：若 `!conn.IsAlive(timeout)` → `OnRemoteNotAlive`（默认：优雅断开）；否则按 `beatFunc` 发送心跳消息（默认发送心跳帧）。
- 服务端同时监听客户端心跳帧（默认路由收到即刷新存活）。
- 客户端侧（`Client`）同样内置心跳发送。
- 修复原实现的明显 bug：`SetHeartBeatMsgFunc`/`SetHeartbeatFunc` 中 `if msgFunc == nil` 判断写反，改为 `!= nil`。

### 6.7 满连接拒绝（有反馈的断开）

- 配置 `MaxConn`；`ConnManager` 维护原子计数（或加锁检查 + Add 原子完成），消除 accept 与 Add 之间的竞态。
- 超限时行为（可配置）：
  1. 默认：向客户端发送预定义"服务器已满"错误消息（`ErrServerFullMsgID`，内容可配），随后优雅关闭；
  2. `OnConnRejected` 钩子（记录来源、指标）；
  3. 计数指标 `conns_rejected`。
- 禁止默默 Close（现状）与裸写响应（半包风险）的做法。

### 6.8 日志（重写 `klog`）

- `ILogger` 接口扩展：`Debug/Info/Warn/Error` 及 `With(fields)`（结构化上下文），保留 `InfoF/ErrorF` 兼容形式。
- 默认实现基于标准库 `log/slog`：级别可配（DEBUG/INFO/WARN/ERROR）、输出可配（stdout/文件/io.Writer）、格式可选（text/JSON）、**无硬编码颜色**。
- 提供**环形缓冲日志后端**（带容量上限），供 kmcp 的 `get_logs` 工具读取最近日志（见 §6.13）。
- 提供 `klog.SetDefault` 全局默认 + `Option` 注入 `Server` 级 logger。
- 框架内部所有 `fmt.Printf` 日志迁移到 `klog`（保留 Accept 等关键事件的结构化字段：remote addr、connID、msgID 等）。

### 6.9 错误处理与 panic 恢复

- 哨兵错误：`ErrServerClosed`、`ErrConnClosed`、`ErrTooLargePacket`、`ErrServerFull`、`ErrProtocol`、`ErrTimeout`。
- `MsgHandler` 执行 Router 时 `defer recover()`：记录堆栈、计数 `handler_panics`、不中断其他消息。
- 读写 goroutine 与 Worker 同规则；`FrameDecoder` 不再 panic。
- 所有 `panic("implement me")` 在 P1/P2 消灭。

### 6.10 指标（`kmetrics`，轻量可观测性）

- 原子计数器/直方图，零外部依赖：
  - `conns_total`（累计创建）、`conns_active`（当前活跃）、`conns_closed`、`conns_rejected`；
  - `msgs_received`、`msgs_sent`、`msgs_dropped`、`bytes_in`、`bytes_out`；
  - `handler_panics`、`heartbeat_missed`、`queue_full`；
  - 直方图：`msg_handle_duration`、`conn_lifetime`、`buffer_pool_hit`。
- 暴露方式：kmcp 工具/资源读取（P4）+ 可选 Prometheus 文本端点（`/metrics`，独立小 HTTP listener，P3）。
- 不引入重量级监控依赖；如需 OpenTelemetry，通过 `IMetrics` 接口后续扩展。

### 6.11 TLS（P3）

- `Option` 注入 `*tls.Config`；Server 侧 `tls.Listen` 包装 listener；Client 侧支持 TLS 拨号。
- 仅作为可选能力，默认纯 TCP 不变。

### 6.12 Client 重写（P3）

- 修复核心缺陷：实现真正 `dial`（`net.Dialer.DialContext`，连接超时可配）。
- 自动重连：指数退避 + 抖动，`Restart/Stop` 语义明确（Stop 后不再自动重连）。
- 与 Server 同构的消息管线：bufio+Decoder（缓冲池化）、拦截器、路由、心跳发送、`SendMsg` 非阻塞写。
- 连接状态回调（OnConnStart/OnConnStop）语义与 Server 对齐。

### 6.13 MCP Server（新包 `kmcp`，P4）

**定位**：把运行中的 Kinz 服务器暴露给 AI 工具（Claude Desktop、IDE、任意 MCP 客户端），实现"AI 开发友好"的运行时闭环。

- **协议**：MCP（Model Context Protocol），JSON-RPC 2.0 子集；实现 `initialize` 握手、`tools/list`、`tools/call`、`resources/list`、`resources/read`。
- **传输**：v1 支持 stdio（Claude Desktop 原生支持）与 TCP 两种 transport，通过 `Transport` 接口抽象，后续可平滑接入官方/第三方 SDK，**v1 零新增外部依赖**（手写 JSON-RPC 编解码，量小可控）。
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
  - `srv.AttachMCP(kmcp.WithTCP(addr))`：开启 TCP MCP 端点；
  - `kmcp.NewServer(srv).RunStdio()`：stdio 模式（给 Claude Desktop 配置文件指向可执行文件）。
- 权限模型：工具调用默认放行（开发/内网运维场景），预留鉴权回调接口（`WithAuth(func(call) error)`）。
- **AI 友好文档**：`docs/mcp.md` 说明如何在 Claude Desktop / IDE 中配置接入、可用的工具与资源清单、示例对话。

### 6.14 文档与 AI 友好（P5）

- **CLAUDE.md 重写**：架构总览、构建/测试命令、代码约定（接口先行、错误处理、命名、缓冲池使用规则）、目录导航、常见坑。
- **`docs/` 目录**：
  - `architecture.md`：包结构、数据流、生命周期图、约定与扩展点清单；
  - `protocol.md`：TLV 线协议与 LengthField 帧格式、字节序、示例报文；
  - `configuration.md`：kconf 全字段说明 + YAML/env 示例；
  - `getting-started.md`：快速开始（服务端/客户端最小示例）；
  - `mcp.md`：MCP Server 配置与使用；
  - `testing.md`：测试命令、模糊测试、基准测试；
  - `faq.md` 与 `production-checklist.md`（部署/压测/调参）。
- **examples/（新 API）**：`ping`（最小回显）、`chatroom`（广播 + 心跳 + 满连接拒绝演示）、`auth-middleware`（拦截器链）、`mcp-stdio`（stdio MCP 接入演示）。
- **代码注释规范**：所有导出符号补齐英文 doc comment（AI 与国际化友好）；中文保留在 README 与设计文档。
- `INTERVIEW_GUIDE.md` 保留并在 P6 按新架构同步更新（属用户个人资产）。
- 原 demo / mmo_game_zinx **不迁移**：按决策在 P1 删除归档（git 历史可回溯）。

### 6.15 测试与 CI（P0 骨架 + 每步交付）

**规则：每一步（P1–P6）的功能与其测试同批落地、同批提交，并在该步退出标准中明确列出。** CI 从 P0 起全程运行，逐步从"允许标红"过渡到全绿。

- **单元测试**：kconf（加载链/缺文件不 panic/YAML 解析）、framedecoder（全分支）、buffer pool（尺寸分级/归还/哨兵）、connmanager、heartbeat（超时/存活/自定义回调）、router & slices（中间件顺序/Abort/Group）、request（上下文/Copy）、msgHandler（分发/panic 恢复）、connection 生命周期（`net.Pipe` 驱动读写）。
- **集成测试**（真实 TCP loopback）：echo 往返、粘包半包重组、心跳超时断开、满连接拒绝（收到错误消息）、优雅停机（在途消息写完）、TLS 握手、Client 重连。
- **模糊测试**：`FrameDecoder.Decode`（`go test -fuzz`）。
- **基准测试**：Pack/Unpack、缓冲池命中率、单连接吞吐、多连接并发。
- **CI**（GitHub Actions）：`go vet` + `go test -race ./...` + `golangci-lint` + 覆盖率门禁（核心包 ≥ 70%）+ 构建产物检查。本地配套 Makefile/Taskfile 命令。

## 7. 分阶段实施计划（每阶段独立验收，顺序执行）

> 每个阶段的"新增测试"与功能同批交付，并在退出标准中验证。

### Phase 0 — 基线锁定（P0）
- 内容：添加 CI 骨架（build + vet 检查，允许标红）、记录当前构建失败清单、整理 todo。
- 退出标准：CI 配置文件落地；构建失败清单与本文档 §3.1 一致。

### Phase 1 — 改名 + 接口重定义（P1）
- 内容：`git rm demo/ mmo_game_zinx/`（删除归档）；module 改名 `kinz`，包改名 `kiface/knet/klog/kconf/kinterceptor`（`utils` 移除）；按新哲学重写全部接口（删除僵尸方法 `Inotify/IFuncRequest/HandleFunc` 等；明确 `IServer/IConnection/IRequest/IMessage/IMsgHandle/IConnManager/IClient` 语义与注释）；引入 `gopkg.in/yaml.v3`；旧实现以返回零值/`ErrNotImplemented` 的占位过渡；清理 `REPRO/TODEL` 注释。
- 新增测试：包级冒烟（各包可编译、接口断言 `var _ kiface.IServer = (*knet.Server)(nil)` 等）。
- 退出标准：`go build ./...` 绿；`kiface` 无僵尸方法；`knet` 无 `panic("implement me")`；所有接口有英文注释；接口断言测试通过。

### Phase 2 — 核心实现重写（P2）
- 内容：`Server`（Run/Shutdown/Serve+信号）、`Connection`（状态机/缓冲写/存活/错误处理/缓冲池化）、消息管线（bufio+Decoder+拦截器接入）、`MsgHandler`（RouterSlices/中间件/工作池优雅关闭/panic 恢复）、心跳接通、满连接拒绝、`Request` 完整实现、kconf 加载链（YAML）。
- 新增测试：kconf 加载链、buffer pool、connection 生命周期（net.Pipe）、心跳超时断开、满连接拒绝、中间件/Group/Abort、优雅停机、echo 集成。
- 退出标准：`examples/ping` 用新 API 跑通；上述新测试全绿（含 `-race`）；SIGTERM 优雅停机验证通过。

### Phase 3 — 生产化（P3）
- 内容：klog 重写（slog + 环形缓冲）、全部日志迁移、哨兵错误、kmetrics + Prometheus 端点、TLS、Client 完整重写（重连/心跳/超时）。
- 新增测试：日志级别/格式、指标计数、TLS 握手、Client 断线重连、错误哨兵断言。
- 退出标准：日志结构化可查；panic 处理器不炸进程；指标可读；TLS 握手验证通过；Client 断线重连验证通过；新测试全绿。

### Phase 4 — MCP Server（P4）
- 内容：`kmcp` 包（协议、stdio/TCP transport、工具与资源）、`AttachMCP` 接入、鉴权回调、环形日志后端接线、`docs/mcp.md`。
- 新增测试：JSON-RPC 编解码、握手、工具调用（用 mock 的 Server Runtime 接口）、stdio 传输。
- 退出标准：任意 MCP 客户端（或手写最小客户端）能完成握手、列出工具、读取连接/指标、向连接发消息、广播、关连接、触发停机。

### Phase 5 — 文档与 AI 友好（P5）
- 内容：CLAUDE.md 重写、`docs/` 全套、导出符号英文注释补齐、examples/（ping/chatroom/auth-middleware/mcp-stdio）。
- 新增测试：examples 作为冒烟测试纳入 CI（build + 启动 + 退出）。
- 退出标准：文档与代码一致；示例可运行并被 CI 构建；注释规范抽查通过。

### Phase 6 — 测试补强 + 发布（P6）
- 内容：单元/集成/模糊/基准测试补全、`INTERVIEW_GUIDE.md` 更新、CI 全绿、覆盖率门禁、CHANGELOG、v1.0.0 tag。
- 新增测试：模糊测试、基准测试基线、剩余边界用例。
- 退出标准：`go test -race ./...` 全绿；核心包覆盖率 ≥ 70%；fuzz 与 bench 可运行并有基线；CI 全绿。

## 8. 兼容性与迁移

- 保留默认 TLV（小端）线协议不变，保证老客户端可通信；`kiface` 中声明 `KinzDataPack`（大端）作为可选。
- API 破坏点明确列出：`NewServer` → `NewServer(opts...)`；`Serve()` → `Serve(ctx)`/`Run(ctx)`；`utils.GlobalObject` → `kconf`；`SendMsg` 返回错误语义不变。
- 原 demo / mmo_game_zinx 删除归档（git 历史可回溯），**不迁移**；新示例在 P2/P5 逐步建立。
- 命名映射：`zinx→kinz`、`ziface→kiface`、`znet→knet`、`zlog→klog`、`zinterceptor→kinterceptor`、`utils→kconf`（新增）、`zmcp→kmcp`（新增）。

## 9. 风险与取舍

| 风险 | 应对 |
|------|------|
| 大改 API 且删除 demo/mmo | git 历史可回溯；examples/ 以新 API 重建冒烟示例 |
| MCP 协议/SDK 生态变化 | transport 抽象隔离，v1 零依赖手写实现，可换 SDK |
| 重写范围大、回归风险 | 接口先行（P1）+ 测试随步交付（每阶段退出标准含新测试）+ 独立验收 |
| Worker 队列打满 | v1 阻塞背压 + `queue_full` 指标；预留丢弃/淘汰策略扩展点 |
| 心跳误杀慢业务 | 存活判定基于"任意消息"而非仅心跳帧；超时参数可配；另有 ReadIdleTimeout 可选 |
| sync.Pool 误用（缓冲被业务持有） | 只池化框架内短期缓冲；归还统一在连接关闭路径；哨兵字节检测误用；文档明确规则 |
| YAML 依赖 | yaml.v3 为唯一运行时外部依赖，成熟稳定；后续可提供内置默认配置免除依赖 |

## 10. 验收标准（Definition of Done）

1. `go build ./...`、`go vet ./...`、`go test -race ./...` 全绿；核心包覆盖率 ≥ 70%。
2. 心跳、满连接拒绝、优雅停机、TLS、Client 重连均有集成测试覆盖；模糊与基准测试可用。
3. 每个阶段的新功能与其测试同批交付（git 提交可追溯）。
4. MCP Server 可被标准 MCP 客户端接入并完成监控/操控演示。
5. `docs/` 与 CLAUDE.md 完整且与代码一致；导出符号有英文注释；examples/ 可运行。
6. 框架内无 `panic("implement me")`、无裸 `panic` 表达协议错误、无 `fmt.Printf` 日志残留。
7. 包名与命名统一为 `kinz/kiface/knet/klog/kconf/kmetrics/kmcp`；`utils`、原 demo、原 mmo_game_zinx 已归档删除。
