# Kinz FAQ

## 消息处理与顺序

**Q: 同一条连接的消息顺序有保证吗？**
有。Worker 池按 `ConnID % WorkerPoolSize` 分配，同一连接永远进同一 Worker，顺序执行。把 `WorkerPoolSize` 设为 0 会退化为每消息一个 goroutine（无顺序保证、吞吐更高）。

**Q: 为什么不用 bufio？**
`ICodec` 自带帧重组缓冲（处理粘包/半包），bufio 是多余的一层分配；读缓冲直接来自 `kpool` 池化。

**Q: 中间件为什么必须手动调 `RouterSlicesNext()`？**
gin 同款语义——中间件可以决定是否放行（不放行 = 拦截），`RouterSlicesNext()` 之后的代码是"业务处理完之后的钩子"（洋葱模型 after）。

**Q: 队列满了会怎样？**
`SendMsgToTaskQueue` 阻塞发送（背压保序），并计数 `kinz_queue_full_total`；业务侧的 `SendMsg` 在写队列满时按 `WriteTimeout` 超时返回 `ErrTimeout`。

## 协议与编解码

**Q: 怎么用自定义协议（JSON/Protobuf/自定义二进制）？**
实现 `kiface.ICodec`（`Decode` 帧重组+解析、`Pack` 封包、`Clone` 每连接实例），`s.SetCodec(codec)` 注入。`Decode` 返回的消息 payload 必须独立复制（会被异步处理）。

**Q: 大端还是小端？**
默认 TLV 小端（兼容旧协议）；`knet.NewTLVPackWithOrder(binary.BigEndian, max)` 切换。字节序是线协议决策——显式声明，不做主机字节序探测。

**Q: 收到超限包会怎样？**
`MaxPacketSize` 超限 → `Decode` 返回 `ErrTooLargePacket` → 连接关闭（fail-fast 安全姿态）。

## 连接与生命周期

**Q: `Stop()` 会阻塞吗？**
会等 Reader/Writer goroutine 退出；Reader 内部路径用非阻塞 `stopOnce`，不会自锁。`Stop()` 幂等（`sync.Once`）。

**Q: 优雅停机流程？**
`Shutdown(ctx)`：停 accept → 排空所有连接（`ClearConn`，受 ctx 超时约束）→ 停 Worker 池（排空队列）→ 关 metrics 监听。`Serve(ctx)` 收到信号后自动执行。

**Q: 连接怎么存业务数据？**
`conn.SetProperty/GetProperty/RemoveProperty`（RWMutex 保护）；请求级临时数据用 `req.Set/Get` 上下文。

## 客户端

**Q: 断线会自动重连吗？**
会。默认 500ms 初值、5s 上限、×2 指数退避 + 抖动；`Stop()` 后不再重连。`WithReconnect(initial, max, multiplier)` 自定义。

**Q: `Start()` 返回什么？**
首次 dial 失败返回 error（不自动重试）；成功后阻塞管理连接（断线重连）直到 `Stop()`。

## 可观测性

**Q: 有哪些指标？**
连接（`kinz_conns_total/active/closed_total/rejected_total`）、消息（`kinz_msgs_received_total/sent_total`）、字节（`kinz_bytes_in_total/out_total`）、处理器（`kinz_handler_panics_total`、`kinz_msg_handle_duration_seconds` 直方图）、背压（`kinz_queue_full_total`）、心跳（`kinz_heartbeat_missed_total`）。`s.AttachMetrics(":9000")` 暴露 `/metrics`（promhttp 标准格式）。

**Q: AI 工具怎么接入？**
MCP（可选）：`kmcp.NewServer(s, ...).ServeHTTP(":9001")`（streamable HTTP）或 `ServeStdio()`；工具包括查看连接/指标/配置/日志、发消息、广播、关连接、停机。见 `docs/mcp.md`。

## TLS 与安全

**Q: 怎么开 TLS？**
`knet.NewServer(knet.WithTLS(tlsCfg))`；客户端 `knet.WithTLSClient(tlsCfg)`。`IConnection.GetConn()` 返回 `net.Conn`（TLS 下是 `*tls.Conn`）。

**Q: MCP 默认无鉴权？**
是（面向开发/内网）。生产必须 `kmcp.WithAuth` 拦截，至少保护 `close_connection`/`shutdown_server`。
