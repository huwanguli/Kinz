# Kinz

> 一个用 Go 编写的、**面向生产环境**的轻量级 TCP 服务端框架 —— 把连接管理、消息编解码、路由分发、并发调度、心跳保活封装起来，业务方只需注册路由函数。

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Coverage](https://img.shields.io/badge/coverage-core%20%E2%89%A5%2070%25-success)]()
[![Tests](https://img.shields.io/badge/tests-unit%20%2B%20integration%20%2B%20fuzz%20%2B%20bench-blue)]()
[![Protocol](https://img.shields.io/badge/protocol-TLV-orange)]()
<!-- 公开仓库后把上面的静态徽章换成 GitHub Actions 徽章：https://github.com/<owner>/<repo>/actions/workflows/ci.yml/badge.svg -->

Kinz 源自教学框架 [zinx](https://github.com/aceld/zinx) 的生产化重写：**砍掉冗余抽象，补齐生产组件，建立明确扩展点（seam）**。设计哲学是「**约定为主 + 明确扩展点**」——默认 TLV 线协议、默认 Worker 池调度开箱即用；编解码、中间件、日志、指标均可按 seam 替换。

---

## ✨ 特性

**核心运行时（`knet`）**

- **TLV 编解码**（`kiface.ICodec` seam）：默认小端 `[DataLen:4][MsgID:4][Data]`，`NewTLVPackWithOrder` 可切大端；流式解码处理粘包/半包，payload 独立复制保证异步安全
- **函数式路由 + 中间件**：`AddRouterSlices(msgID, handlers...)`、`Group(start, end, ...)` 区间分组、`Use(...)` 全局中间件；洋葱模型 before/after 语义 + `Abort`
- **Worker 工作池**：固定 Goroutine 数、按 ConnID 保序、阻塞背压（不丢消息）、优雅排空关闭
- **连接状态机**：created → running → closed，`sync.Once` 幂等关闭；`msgChan` 永不 close（无关闭 panic）；Reader 不死锁
- **心跳检测**：存活判定基于任意消息（慢业务不误杀）；默认心跳 MsgID `99999`；超时回调默认优雅关连接
- **满连接拒绝**：超 `MaxConn` 时新连接收到保留 MsgID `0xFFFFFFFE` 错误帧后断开
- **优雅停机**：`Serve(ctx)` / `Shutdown(ctx)`，在途请求排空，幂等

**生产化组件**

- **指标（`kmetrics`）**：基于 Prometheus `client_golang`，`AttachMetrics(":9000")` 暴露 `/metrics`；热路径埋点预取指针、零 map 查找
- **TLS**：`WithTLS(*tls.Config)`，连接层统一 `net.Conn` 抽象
- **Client**：完整客户端，指数退避 + 抖动自动重连、心跳、路由，与 Server 共用连接生命周期
- **配置（`kconf`）**：加载链 `默认值 → kinz.yaml → KINZ_* 环境变量`，缺文件不报错
- **日志（`klog`）**：`log/slog` 结构化日志 + 环形缓冲后端（供 MCP `get_logs`）
- **MCP（`kmcp`，可选）**：把运行时暴露给 AI 工具——10 个工具 + 4 个资源，stdio（Claude Desktop）/ streamable HTTP 双 transport，`WithAuth` 鉴权

---

## 🚀 快速开始

```bash
go get kinz          # module 名：kinz（推送到 GitHub 后改为 github.com/<owner>/kinz）
```

**最小服务端（echo）**

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kinz/kiface"
	"kinz/klog"
	"kinz/knet"
)

func main() {
	s := knet.NewServer()

	// msgID 1 (ping) -> msgID 2 (pong)，回显载荷
	if _, err := s.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		panic(err)
	}

	// 全局计时中间件：RouterSlicesNext() 之后的代码在业务处理完后执行
	if _, err := s.Use(func(req kiface.IRequest) {
		start := time.Now()
		req.RouterSlicesNext()
		klog.L().Info("handled", "msgID", req.GetMsgID(), "elapsed", time.Since(start).String())
	}); err != nil {
		panic(err)
	}

	s.StartHeartBeat(10 * time.Second) // 心跳
	s.AttachMetrics(":9000")           // Prometheus 指标端点

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	klog.L().Info("starting")
	if err := s.Serve(ctx); err != nil { // 优雅停机
		panic(err)
	}
}
```

**最小客户端**（自动重连）

```go
c := knet.NewClient("127.0.0.1", 8999, knet.WithReconnect(500*time.Millisecond, 3*time.Second, 2))

pongs := make(chan string, 1)
_, _ = c.AddRouterSlices(2, func(req kiface.IRequest) { pongs <- string(req.GetData()) })

if err := c.Start(); err != nil { panic(err) } // dial；断线后自动重连
_ = c.Conn().SendMsg(1, []byte("hello"))        // 发 ping
got := <-pongs                                  // 收 pong
c.Stop()
```

**换协议**：实现一个 `kiface.ICodec`（`Decode` / `Pack` / `Clone`），`s.SetCodec(codec)` 即可，业务代码零改动。

---

## 🔌 线协议

```
[DataLen: 4 字节][MsgID: 4 字节][Data: DataLen 字节]
```

- 默认小端序；`NewTLVPackWithOrder(binary.BigEndian, n)` 可配大端
- 保留 MsgID：`99999`（心跳）、`0xFFFFFFFE`（满连接拒绝）
- `DataLen` 超 `maxPacketSize` → 返回哨兵错误 `ErrTooLargePacket`（防恶意大包）
- 详见 [`docs/protocol.md`](docs/protocol.md)

---

## 🏗️ 架构

```
┌──────────────────────────── 业务层 ────────────────────────────┐
│   RouterHandler（路由函数） · Use/Group（中间件） · 自定义 ICodec  │
└──────────────────────────── kiface 契约层 ─────────────────────┘
        │                                    │
   ┌────▼─────┐  ┌─────▼──────┐  ┌─────▼─────┐  ┌─────▼──────┐
   │  knet    │  │   klog     │  │  kconf    │  │  kmetrics  │
   │ Server/  │  │ slog+环形   │  │ YAML+env  │  │ client_go- │
   │ Client/  │  │ 缓冲       │  │ 配置链    │  │ lang 适配  │
   │ Conn/    │  └────────────┘  └───────────┘  └────────────┘
   │ MsgHandler│
   └────┬─────┘
   ┌────▼─────┐  ┌─────▼──────┐
   │  kpool   │  │   kmcp     │ ← 可选：MCP 服务器（应用接线，核心不依赖）
   │ sync.Pool│  │ mcp-go SDK │
   └──────────┘  └────────────┘
```

**包结构**

| 包 | 职责 |
|----|------|
| `kiface` | 纯契约层：接口 + 9 个哨兵错误（零实现） |
| `knet` | 运行时：Server / Client / Connection / MsgHandler / ConnManager / TLVPack |
| `klog` | `ILogger` + slog 实现 + 环形缓冲后端 |
| `kconf` | 配置加载链：默认值 → YAML → `KINZ_*` env |
| `kpool` | 4K / 16K / 64K 分级 `sync.Pool` 字节缓冲 |
| `kmetrics` | Prometheus `client_golang` 适配（写句柄 + `Snapshot()` + promhttp） |
| `kmcp` | **可选** MCP 服务器（mark3labs/mcp-go），10 工具 + 4 资源 |

**扩展点（seam）**：`ICodec`（换协议）· `RouterHandler`（横切逻辑）· `ILogger`（换日志）· `kmetrics.Registry`（换指标后端）· `kmcp`（可选接线）。接口只出现在真正会变的地方——重构中砍掉了与中间件重叠的拦截器链、与函数路由重叠的经典 `IRouter`。

详细设计见 [`docs/architecture.md`](docs/architecture.md)。

---

## 📊 性能

本机实测（i7-12700H / Windows 11 / go1.26，完整数据与复现命令见 [`docs/performance.md`](docs/performance.md)）：

| 场景 | 结果 |
|------|------|
| TLV Pack（4KB） | **2.6 GB/s** |
| TLV Decode（4KB） | **1.2 GB/s**（payload 独立复制，异步安全） |
| 路由分发（worker 池全路径） | **220 ns/op，零分配** |
| 指标埋点 | **6.3 ns/op** |
| 单连接 echo 吞吐 | **~7–7.5k msg/s**（延迟受限，16B 与 1KB 耗时几乎相同） |
| 32 连接聚合吞吐 | **~54k msg/s / 6.9 MB/s**（128B payload） |

结论：单连接延迟受限（并发是吞吐杠杆）；框架相对裸 TCP 开销 ≈2.2×（decode + 分发 + Pack + 队列跳转）；每消息 ~15 次分配，优化方向已按收益排序写入文档。

---

## ✅ 质量保障

- **测试**：单元（codec 粘包/半包/大端/payload 独立、中间件顺序/Abort/Group、状态机、心跳、池、配置链）+ 集成（真实 TCP：echo、满连接拒绝、心跳超时、优雅停机、TLS、Client 重连、指标端点、MCP 全链路）
- **模糊测试**：`FuzzTLVPackDecode` / `FuzzRingBuffer` / `FuzzLoadYAML`
- **基准测试**：20+ 项，基线落档 `docs/performance.md` 防回归
- **覆盖率门禁**：核心包 ≥ 70%（kpool 100% / klog 96.3% / kmetrics 89.6% / kconf 86.2% / knet 74.0% / kmcp 82.4%）
- **CI**（GitHub Actions）：build + vet + `go test -race` + 覆盖率门禁 + fuzz 冒烟 + bench 冒烟

---

## 📦 运行示例

```bash
go run ./examples/echo/server            # echo 服务端（8999 + metrics :9000 + MCP :9001/mcp）
go run ./examples/echo/client            # echo 客户端（3 次 ping-pong）
go run ./examples/chatroom/server        # 聊天室广播（join/leave 钩子 + 心跳）
go run ./examples/chatroom/client
go run ./examples/auth-middleware/server # Use 中间件鉴权演示
go run ./examples/auth-middleware/client
go run ./examples/mcp-stdio              # MCP stdio 桥（Claude Desktop 可指向）
```

---

## 📚 文档

| 文档 | 内容 |
|------|------|
| [getting-started](docs/getting-started.md) | 快速开始 + API 速查 |
| [architecture](docs/architecture.md) | 包结构、数据流、生命周期、扩展点 |
| [protocol](docs/protocol.md) | TLV 线协议、字节序、自定义 ICodec |
| [configuration](docs/configuration.md) | 配置字段、YAML、`KINZ_*` env |
| [performance](docs/performance.md) | 基准测试基线、瓶颈分析、优化建议 |
| [testing](docs/testing.md) | 测试命令、fuzz/bench 用法 |
| [mcp](docs/mcp.md) | MCP 接入、工具/资源清单 |
| [faq](docs/faq.md) | 常见问题 |
| [production-checklist](docs/production-checklist.md) | 部署、调参、监控、安全 |
| [CHANGELOG](CHANGELOG.md) | 变更记录（v1.0.0） |

---

## 📁 目录结构

```
kiface/        契约层（接口、哨兵错误）
knet/          运行时（Server、Client、Connection、MsgHandler、TLVPack…）
klog/          slog 日志 + 环形缓冲
kconf/         配置（默认值 / YAML / env）
kpool/         sync.Pool 分级缓冲
kmetrics/      Prometheus 指标适配
kmcp/          可选 MCP 服务器
examples/      可运行示例（echo / chatroom / auth-middleware / mcp-stdio）
docs/          文档全套
```

## 📄 License

尚未指定。公开仓库前请补充 LICENSE 文件并更新此处。

---

*Kinz 由教学框架 [zinx](https://github.com/aceld/zinx) 重构而来（原 demo 与 mmo_game_zinx 已归档于 git 历史，可回溯）。AGENT.md 为本仓库的 AI 协作指南。*
