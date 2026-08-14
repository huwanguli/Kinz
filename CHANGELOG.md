# Changelog

本项目（原 zinx，重构后更名 **Kinz**）所有重要变更记录于此。格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 新增

- `README.md`：GitHub 主页入口文档（特性、快速开始代码、架构图、性能概览、质量保障、示例与文档导航）

## [v1.0.0] - 2026-08-14

### 发布（P6：测试补强 + 发布）

**新增模糊测试**（`go test -fuzz`，均通过冒烟无崩溃）：

- `knet.FuzzTLVPackDecode`：任意字节流喂入 TLV 解码器，不得 panic/崩溃，且解析出的消息自洽（dataLen 与 payload 一致）
- `klog.FuzzRingBuffer`：任意字节序列 + 任意容量，Write/Bytes/String/Lines 不越界
- `kconf.FuzzLoadYAML`：任意 YAML 内容经配置加载链不得 panic

**新增基准测试**（完整基线落档 `docs/performance.md`，i7-12700H / Win11 / go1.26 实测）：

- 编解码微基准：`TLVPack` Pack 16B–4KB（143ns–1.58μs，最高 2.6 GB/s）、Decode（200ns–3.4μs，最高 1.2 GB/s）、粘包流解析（≈206ns/帧）
- 分发微基准：`MsgHandler.Execute` 78.6ns、worker 池全路径 `SendMsgToTaskQueue` 220ns 零分配
- 端到端吞吐（真实 TCP 环回）：单连接 echo 7–7.5k msg/s（延迟受限，~130–145μs/往返）；多连接聚合 8 连接 38.7k、32 连接 53.7k msg/s；新增裸 TCP 基线（62μs/往返）量化框架开销 ≈2.2×
- 基础设施：kpool 热池往返 ~30ns、冷池 1.16μs、越级直配 17μs；klog 环形缓冲写 229ns 零分配；kmetrics 埋点 6.3ns 零分配；kconf 加载链 55μs（启动期一次性）

**文档**：

- 新增 `docs/performance.md`：环境、方法论、全量指标表、瓶颈分析（延迟受限、并发是吞吐杠杆）、每消息分配账本（15 allocs/~960B）、按收益排序的优化建议、复现命令
- 重写 `docs/INTERVIEW_GUIDE.md`：按新架构同步（函数式路由、中间件洋葱模型、状态机、Worker 背压、MCP、性能数据），删除过时内容（拦截器链、经典 IRouter、MMO/AOI）
- `docs/testing.md`：fuzz/bench 现状与命令、覆盖率更新（kmcp 65.8% → 82.4%）

**工程**：

- `kmcp` 覆盖率提升：`WithVersion`、`get_connection`（含错误路径）、`broadcast`（双连接实测送达）、`ServeHTTP`（真实端口 listener）、`ServeStdio`（重构出可测的 `serveStdio(in,out)` seam）；`ServeHTTP` 端点路径 `/mcp` 与 mcp-go 默认一致
- CI（GitHub Actions）：build + vet + `go test -race -cover ./...`（Linux 上跑 race）；`Makefile` 增加 bench/fuzz/覆盖率门禁目标
- 覆盖率门禁达成：核心包 kpool 100% / klog 96.3% / kmetrics 89.6% / kconf 86.2% / knet 74.0% ≥ 70%；可选包 kmcp 82.4%

### 修复

- `BenchmarkMsgHandlerDispatch` 结束断言竞态：改为先 `StopWorkerPool`（排空队列）再校验处理数
- `ServeHTTP` 集成测试 4xx：客户端须以 `/mcp` 路径访问（`Start` 默认挂载点，与 `Handler()` 裸 handler 不同语义）

## 重构历程（P0–P5，v1.0.0 之前）

### [P5] 文档与 AI 友好 - 2026-08-14

- 新增 `docs/` 全套：`architecture.md`（包结构/读写路径/生命周期/seam 清单）、`protocol.md`（TLV 线协议/字节序/保留 MsgID）、`configuration.md`（全字段表/env）、`getting-started.md`、`testing.md`、`faq.md`（14 问）、`production-checklist.md`（部署/调参/监控）、`mcp.md`
- `AGENT.md` 重写至 P5 状态；全部导出符号补齐英文 doc comment
- 新增 examples：`chatroom`（广播 + join/leave 钩子 + 心跳）、`auth-middleware`（Use 鉴权中间件）、`mcp-stdio`（Claude Desktop stdio 桥）
- 实测通过：chatroom 广播、auth 三步鉴权流程

### [P4] MCP Server - 2026-08-14

- `kmcp` 包迁移至 **mark3labs/mcp-go** SDK：streamable HTTP（默认 `/mcp`）+ stdio 双 transport
- 改为独立适配器模式（应用接线，`kmcp.NewServer(srv, opts...).ServeHTTP/ServeStdio`），核心 `knet` 不依赖 `kmcp`，避免循环依赖
- 10 工具 + 4 资源（connections:// metrics:// config:// logs://）；`WithAuth` 鉴权回调；`WithConfig/WithLogRing/WithVersion` 选项
- `kmetrics` 改用 **prometheus/client_golang**（写句柄 + `Snapshot()` + promhttp），删除手写文本导出
- `knet` 新增 `IConnManager.Range`；`kconf.Config` 补 json tag

### [P3] 生产化 - 2026-08-14

- `klog` 环形缓冲后端（`RingBuffer`，供 MCP get_logs）；全部日志迁移至 klog
- 哨兵错误体系（`kiface.Errors` 9 个 + `ServerFullMsgID`）
- `kmetrics`：Counter/Gauge/Histogram + `Registry.Snapshot()` + `Server.AttachMetrics(addr)` Prometheus 端点；连接/分发/心跳全链路埋点
- TLS：`WithTLS(*tls.Config)`；`IConnection.GetConn()` 返回 `net.Conn` 抽象
- Client 完整实现：指数退避重连、心跳、路由；新增 `connHost` 抽象与 Server 共用连接生命周期
- 修复 Client 消息乱序 bug（`Start()` 未启动 Worker 池）
- `kconf` 完成 `KINZ_*` env 全覆盖

### [P2] 核心实现 - 2026-08-14

- `Server` 完整生命周期：`Run(ctx)`/`Shutdown(ctx)`/`Serve(ctx)` + 信号处理
- `Connection` 状态机（created→running→closed）+ `sync.Once` 幂等 Stop + `stopOnce` 非阻塞清理（Reader 不死锁）；`msgChan` 永不 close
- 消息管线：`ICodec` 接入 + `MsgHandler`（RouterSlices/中间件/Worker 池优雅关闭/panic 恢复）
- 心跳接通；满连接拒绝（`ServerFullMsgID`）
- **合并 codec seam**：`decode`(IDataPack) 与 `framedecode`(FrameDecoder) 合并为单一 `ICodec`，丢弃 FrameDecoder/LengthField；修复 payload 数据竞争（`Decode` 返回前独立复制）
- **删除拦截器链**（与 Use/Group 中间件功能重叠）与死配置字段
- `kconf`（yaml.v3 加载链）+ `kpool`（4K/16K/64K sync.Pool 分级缓冲）引入

### [P1] 改名 + 接口重定义 - 2026-08-13

- module 更名 `kinz`；`ziface→kiface`、`znet→knet`、`zlog→klog`、`zinterceptor→kinterceptor`（后删除）、`utils→kconf`
- 删除归档 `demo/`、`mmo_game_zinx/`（git 历史可回溯）；`.idea/` 解除跟踪；`CLAUDE.md→AGENT.md`、`INTERVIEW_GUIDE.md` 移入 `docs/`
- 按"约定为主 + 明确扩展点"哲学重写全部接口；删除僵尸方法（`Inotify/IFuncRequest/HandleFunc` 等）
- `klog` 以 `log/slog` 重写（级别/JSON/动态级别/InfoF 兼容）
- **删除经典 IRouter**（PreHandle/Handle/PostHandle），只留函数式 `RouterHandler`

### [P0] 基线锁定 - 2026-08-13

- CI 骨架落地（build + vet）；记录基线构建失败清单（`docs/superpowers/plans/2026-08-14-baseline-build-errors.txt`）
- 设计文档 `docs/superpowers/specs/2026-08-14-zinx-production-refactor-design.md` 与实施计划落地

## 重构前（zinx 历史，2021–2024）

- 教学/课设代码：V0.1–V1.0（基础功能、连接封装、路由、配置、消息封装、多路由、读写协程分离与工作池、Hook、属性）与计网课设（世界聊天、玩家同步、AOI 九宫格）；**不迁移**，已归档于 git 历史（`924d99e`…`82646e9`）
