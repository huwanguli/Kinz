# Kinz 生产部署清单

## 部署

- **单一二进制**：`go build ./...`，交付 `examples/echo/server` 或你自己的 main。
- **systemd 示例**（`/etc/systemd/system/kinz.service`）：

```ini
[Unit]
Description=Kinz server
After=network.target

[Service]
ExecStart=/opt/kinz/server
Restart=on-failure
LimitNOFILE=65536
Environment=KINZ_PORT=9000
Environment=KINZ_MAXCONN=10000

[Install]
WantedBy=multi-user.target
```

- **文件描述符**：TCP 长连接数受 `ulimit -n` 限制；`MaxConn` 建议 ≤ 系统 fd 上限的 80%（留余量给监听/MCP/metrics fd）。
- **优雅停机**：`Serve(ctx)` + SIGTERM（demo 已接）；systemd `KillSignal=SIGTERM`、`TimeoutStopSec` 给足排空时间。
- **TLS**：生产必须启用；证书用正式 CA 或内部 PKI，`tls.Config{MinVersion: tls.VersionTLS12}`。

## 调参

| 参数 | 场景建议 |
|------|---------|
| `MaxConn` | 按 ulimit 与单连接内存预算（每连接 ~几十 KB：读缓冲 4K + 写队列 256×包 + codec 缓冲） |
| `WorkerPoolSize` | 默认 10；CPU 密集业务按 `NumCPU` 调整；I/O 密集可更大。同一连接保序依赖它 > 0 |
| `MaxWorkerTaskLen` | 突发流量缓冲；满则背压（阻塞 reader），观察 `kinz_queue_full_total` |
| `WriteQueueSize` | 单连接写缓冲；下游慢时提高可减少 `SendMsg` 超时，但内存上升 |
| `WriteTimeout` | 对端不读时防写阻塞；默认 5s |
| `MaxPacketSize` | 按业务最大消息定；过小误杀、过大防滥用失效 |
| 心跳 | `StartHeartBeat(interval)`；超时默认 3×interval；生产建议显式 `SetHeartBeatWithOption` 设超时 |

## 监控

- **指标**：`s.AttachMetrics(":9000")` → Prometheus 抓取 `/metrics`；或集成到已有 mux：`mux.Handle("/metrics", s.GetMetrics().Handler())`。
- **关键告警**：
  - `kinz_conns_active` 接近 `MaxConn`（扩容或调参）
  - `kinz_queue_full_total` 快速增长（worker 吃紧）
  - `kinz_handler_panics_total` 增长（业务 bug）
  - `kinz_heartbeat_missed_total` 增长（对端失联/网络问题）
  - `kinz_msg_handle_duration_seconds` 直方图 p99 超预期
- **MCP（可选）**：AI 工具实时查看连接/指标/日志、发消息、控制停机——生产按 `docs/mcp.md` 配好 `WithAuth`。

## 压测

- 用框架自己的 `knet.Client` 写压测客户端，或 `vegeta`/自研并发拨号。
- 关注：QPS、p99 延迟、`conns_active` 峰值、GC 压力（`GODEBUG=gctrace=1`）、fd 占用。
- 基线建议：先单连接吞吐，再 `MaxConn` 满额并发，观察背压指标是否触发。

## 安全清单

- [ ] TLS 启用（MinVersion ≥ TLS1.2）
- [ ] `MaxPacketSize` 设业务上限
- [ ] MCP `WithAuth`（若开启）；`shutdown_server`/`close_connection` 必须授权
- [ ] 满连接拒绝有反馈（默认已实现：`ServerFullMsgID` + 关闭）
- [ ] 日志脱敏（klog 结构化，勿记 token/密钥）
- [ ] 心跳兜底（对端假死连接自动回收）
- [ ] panic 恢复默认开启（单消息失败不炸进程）
