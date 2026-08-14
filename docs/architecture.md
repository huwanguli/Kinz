# Kinz 架构

Kinz 是一个 Go 编写的 TCP 服务器框架。设计哲学是**约定为主 + 明确扩展点（seam）**：框架约定"管线的形状"（生产安全默认值开箱即用），业务决定"管线的内容"（通过有限的 seam 扩展）。

## 包结构

```
kiface/        契约层：全部接口 + 哨兵错误（ErrServerClosed/ErrConnClosed/ErrTooLargePacket/...）
knet/          实现层：Server / Client / Connection / MsgHandler / ConnManager /
               HeartBeatChecker / TLVPack（ICodec 默认实现）/ Message / Request
klog/          ILogger 接口 + slog 默认实现 + RingBuffer（供 MCP get_logs）
kconf/         Config（默认值 → kinz.yaml → KINZ_* env 加载链）
kpool/         4K/16K/64K 尺寸分级 sync.Pool 缓冲池
kmetrics/      指标层（prometheus/client_golang 适配：Counter/Gauge/Histogram + Snapshot + promhttp Handler）
kmcp/          MCP Server（mark3labs/mcp-go，stdio + streamable HTTP）——可选适配器，核心不依赖
examples/      可运行示例（echo / chatroom / auth-middleware / mcp-stdio）
docs/          本文档 + protocol / configuration / getting-started / testing / faq / mcp / production-checklist
```

依赖：核心 = `gopkg.in/yaml.v3` + `prometheus/client_golang`；`mcp-go` 仅在 import `kmcp` 时链接（MCP 可选）。

## 核心数据流

### 读路径（一条消息的旅程）

```
socket → kpool 池化读缓冲 → ICodec.Decode（帧重组 + TLV 解析，payload 独立复制）
       → IRequest → MsgHandler.SendMsgToTaskQueue（按 ConnID 取模入 Worker 队列，保序 + 背压）
       → MsgHandler.Execute（recover 保护）
            ├─ 中间件链：Use（全局）→ Group（区间）→ 路由 handlers（RouterSlicesNext 续链，洋葱模型）
            └─ 业务 handler
```

- `ICodec` 是有状态、每连接一份（`Clone()`）：内部缓冲处理 TCP 粘包/半包；返回的消息 payload 独立复制，可安全异步处理。
- Worker 池保证**同一连接的消息顺序执行**；队列满时阻塞（背压），计数 `kinz_queue_full_total`。
- 任何 handler panic 都被 `Execute` 恢复（记日志 + 计数 `kinz_handler_panics_total`），不炸进程。

### 写路径

```
业务 → conn.SendMsg(msgID, data) → codec.Pack → msgChan（缓冲 WriteQueueSize，三路 select 带超时）
     → Writer goroutine → socket
```

所有业务线程的发送汇入**单一 Writer goroutine** 串行写 socket，天然无写并发问题。

## 生命周期

```
Server.Run(ctx)    启动 listener（可 TLS）→ Worker 池 → accept 循环 → 阻塞至 ctx 取消 → 优雅停机
Server.Shutdown(ctx)  停止 accept → 排空连接（ClearConn，ctx 超时）→ 停止 Worker 池（排空队列）
Server.Serve(ctx)  = Run（应用负责 signal.NotifyContext）
Client.Start()    首次 dial（失败返回 error）→ 管理连接 + 自动重连（指数退避+抖动）→ Stop 停止
```

## 约定（默认路径，零配置可用）

| 能力 | 默认行为 |
|------|---------|
| 心跳 | `StartHeartBeat(interval)` 开启；超时 3×interval；任何收到消息刷新存活；超时默认优雅断开 |
| 满连接拒绝 | `MaxConn` 超限 → 发送 `ServerFullMsgID`(0xFFFFFFFE) 错误消息 → 关闭 |
| panic 恢复 | 每个消息的 handler panic 被恢复，不中断其他消息 |
| 优雅停机 | `Run` 取消后自动：停 accept → 排空连接 → 停 Worker 池 |
| 日志 | 全走 klog（slog），框架内无 fmt.Printf |
| 指标 | 开箱即用（conns/msgs/bytes/panics/duration/queue_full），`AttachMetrics(addr)` 暴露 /metrics |

## 扩展点（seam）

| seam | 接口 | 默认实现 | 用途 |
|------|------|---------|------|
| 编解码 | `kiface.ICodec` | `knet.TLVPack` | 换协议（分帧+解析+封包一体） |
| 中间件 | `RouterHandler`（Use/Group） | 空 | 鉴权、日志、限流、解密替换消息 |
| 日志 | `klog.ILogger` | slog | 结构化日志、级别/格式/输出 |
| 指标 | `kmetrics.Registry` | client_golang | 计数器/直方图/暴露 |
| 运行时桥 | `kmcp`（可选） | mcp-go | 把运行中服务器暴露给 AI 工具 |

## 关键内部机制

- **Connection**：原子状态机（created→running→closed）+ `sync.Once` 幂等 Stop；`stopOnce` 非阻塞清理（Reader 不死锁），外部 `Stop()` 再 `wg.Wait()`；`msgChan` 永不 close（Writer 靠 done 退出）。
- **connHost 抽象**：`Server` 与 `Client` 共同实现（GetHeartBeat/CallOnConnStart/CallOnConnStop/GetConnMgr），连接生命周期完全复用。
- **缓冲池**：读缓冲从 `kpool` 取（连接建立时 Get、关闭时 Put），避免反复分配。
- **指标埋点**：连接（bytes/msgs/active/closed）、handler（panics/duration/queue_full）、心跳（missed）——计数器在构造时预取指针，热路径零查找。
