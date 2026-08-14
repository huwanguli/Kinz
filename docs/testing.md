# Kinz 测试

## 命令

```bash
go build ./...            # 构建全部（含 examples）
go vet ./...              # 静态检查
go test ./...             # 全部测试
go test -race ./...       # 竞态检测（需要 C 工具链；本机开发机未装时在 CI/linux 上跑）
go test ./knet/ -v        # 某包详情
go test -cover ./...      # 覆盖率
go test ./knet/ -coverprofile=coverage.out && go tool cover -func=coverage.out
```

## 测试分布

| 包 | 覆盖内容 |
|----|---------|
| `knet` | 单元（TLVPack 往返/粘包/半包/大端/超限/payload 独立、MsgHandler 中间件/Abort/Group/panic 恢复、HeartBeat 存活/超时/克隆、ConnManager、重连退避）+ 集成（真实 TCP：echo、粘包、满连接拒绝、心跳超时、优雅停机、TLS、Client 重连、指标端点） |
| `kconf` | 默认值 / 缺文件 / YAML / 非法 YAML / 非法 duration / env 全覆盖 / env 非法值 |
| `kmetrics` | Counter/Gauge/Histogram、Snapshot、去重、promhttp 端点 |
| `klog` | slog 级别/JSON/With/InfoF/动态级别/包级函数/环形缓冲（容量/顺序/Lines/并发） |
| `kpool` | 尺寸分级/复用/哨兵/误尺寸丢弃/大缓冲直配 |
| `kmcp` | 工具注册/调用/鉴权/资源/未知方法（mcp-go client 驱动）+ 真实 server 集成 + stdio 传输 |

## 覆盖率门禁（每阶段）

- 当前实际：kmetrics 89.6% / klog 96.3% / knet 74.0% / kconf 86.2% / kpool 100% / kmcp 82.4%
- P6 门禁：核心包（knet/kconf/kmetrics/klog/kpool）≥ 70% —— **已达标**（kmcp 可选包亦达 82.4%，见 `docs/architecture.md` §可选组件）

## 模糊测试

P6 已落地三个 fuzz target（均通过 5s+ 冒烟，无崩溃）：

```go
// knet/fuzz_test.go —— TLV 解码：任意字节流不得 panic/崩溃，且消息自洽
func FuzzTLVPackDecode(f *testing.F) { ... }
// klog/fuzz_test.go —— 环形缓冲：任意写入 + 任意容量，Lines(n) 不越界
func FuzzRingBuffer(f *testing.F) { ... }
// kconf/fuzz_test.go —— 配置加载：任意 YAML 内容不得 panic
func FuzzLoadYAML(f *testing.F) { ... }
```

运行：

```bash
go test ./knet/ -run '^$' -fuzz=FuzzTLVPackDecode -fuzztime=30s
go test ./klog/ -run '^$' -fuzz=FuzzRingBuffer -fuzztime=30s
go test ./kconf/ -run '^$' -fuzz=FuzzLoadYAML -fuzztime=30s
```

## 基准测试

P6 已落地完整基准集（微基准 + 端到端吞吐 + 裸 TCP 基线），**基线数值与分析方法见 `docs/performance.md`**。概览：

| 基准 | 覆盖 | 关键数值（i7-12700H / Win11 / go1.26） |
|------|------|------------------------------------------|
| `BenchmarkTLVPackPack/Decode` | 编解码微基准（16B–4KB） | Pack 4KB 2.6 GB/s；Decode 4KB 1.2 GB/s |
| `BenchmarkTLVPackDecodeSticky` | 粘包流解析 | ≈206ns/帧 |
| `BenchmarkMsgHandlerExecute` / `Dispatch` | 路由直执行 / worker 池全路径 | 78.6ns / 220ns，零分配 |
| `BenchmarkConnectionSendMsg` | 连接写路径 | ~93μs/条（环回 syscall 主导） |
| `BenchmarkRawEchoBaseline` | 裸 TCP 回显基线 | 128B 往返 62μs |
| `BenchmarkEchoThroughput` | 单连接完整 echo 往返 | ~7–7.5k msg/s（延迟受限） |
| `BenchmarkMultiConnEcho` | 多连接聚合吞吐（1/8/32） | 32 连接 54k msg/s / 6.9 MB/s |
| `BenchmarkPoolGetPut` / 冷池 / 直配 | 缓冲池命中率 | 热池往返 ~30ns；冷池 1.16μs；越级直配 17μs |
| `BenchmarkRingBufferWrite` / `Lines` | 日志环形缓冲 | 写 229ns 零分配；Lines(50) 14μs |
| `BenchmarkLogInfo` | slog 日志（输出丢弃） | 784ns；AddSource 1.39μs |
| `BenchmarkDefault` / `LoadYAML` | 配置加载链 | 启动期 55μs（一次性） |
| `BenchmarkCounterInc` / `Snapshot` | 指标埋点 / 快照 | 埋点 6.3ns 零分配 |

运行：

```bash
go test ./knet/ ./kpool/ ./klog/ ./kconf/ ./kmetrics/ -bench "." -benchmem -benchtime=2s
go test ./knet/ -bench "RawEchoBaseline|EchoThroughput|MultiConnEcho" -benchmem -benchtime=1s -count=3
```

> Windows PowerShell 注意：`-bench "."` 的 `.` 必须加引号，否则被当作别名解析、基准静默不执行。

## 集成测试如何写

- 服务端用 `127.0.0.1:0`（随机端口）+ `srv.Address()` 取真实地址，避免端口冲突。
- 客户端可以是原始 TCP + `knet.NewTLVPack()`（测试编解码与协议），或 `knet.Client`（测试框架客户端）。
- 配置一律从 `kconf.Default()` 派生再覆盖，避免结构体字面量把默认字段清零。
