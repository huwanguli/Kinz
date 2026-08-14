# Kinz 性能基准（v1.0.0）

本文档记录 Kinz 框架的基准测试基线：**测试环境、方法论、各层指标、瓶颈分析与调优建议**。所有数字在本机实测产出，可作为性能回归的对照基线。

## 1. 测试环境

| 项 | 值 |
|----|----|
| CPU | 12th Gen Intel Core i7-12700H（14 核 20 线程，移动平台） |
| 内存 | 32 GB DDR5 |
| OS | Windows 11 家庭中文版（x64） |
| Go | go1.26.0 windows/amd64 |
| GOMAXPROCS | 20（未设置，取默认） |
| 测试日期 | 2026-08-14 |
| 运行方式 | `go test ./<pkg>/ -bench <regex> -benchmem -benchtime=2s`（集成吞吐 `-benchtime=1s -count=3`） |

> 注意：Windows 环回（loopback）上小包往返的调度延迟高于 Linux；跨平台对比时请在同一环境复跑。数值存在 ±10–15% 的 run-to-run 波动（尤其端到端吞吐），集成测试取 3 次中位数。

## 2. 方法论

- **微基准**（codec / 路由 / 缓冲池 / 日志 / 指标）：`-benchtime=2s`，测量单操作耗时与分配。
- **端到端吞吐**（真实 TCP 环回）：服务端 echo 路由（收到 msgID 1 → `SendMsg(2, data)`），客户端发 1 条等 1 条回显，测完整往返（client write → server read/decode → worker 分发 → echo SendMsg → client read）。`-benchtime=1s -count=3`，报告中位数。
- **吞吐单位**：MB/s 统计**应用 payload 字节**；线上字节 = payload + 8 字节头。msg/s 为完整消息往返数。
- **对比基线**：`BenchmarkRawEchoBaseline`——裸 `net.Conn` 读回写（无框架、无协议解析），作为框架开销的下界参照。

## 3. 微基准

### 3.1 TLV 编解码（knet，`BenchmarkTLVPackPack/Decode`）

| 场景 | ns/op | MB/s | B/op | allocs/op |
|------|-------|------|------|-----------|
| Pack 16B | 143 | 168 | 80 | 4 |
| Pack 128B | 179 | 760 | 200 | 4 |
| Pack 1KB | 515 | 2 004 | 1 208 | 4 |
| Pack 4KB | 1 578 | 2 602 | 4 920 | 4 |
| Decode 16B | 200 | 120 | 128 | 5 |
| Decode 128B | 281 | 485 | 360 | 5 |
| Decode 1KB | 977 | 1 057 | 2 264 | 5 |
| Decode 4KB | 3 439 | 1 193 | 9 048 | 5 |
| Decode 粘包流（16×64B/次） | 3 299（≈206/帧） | 349 | 3 824 | 54（≈3.4/帧） |

要点：**Decode 是内存拷贝主导**（为异步安全，payload 与头各自独立复制，见 `docs/protocol.md`），4KB 帧达 1.2 GB/s；Pack 为 `bytes.Buffer` + `binary.Write`，4 次分配。每帧固定 ~3–5 次分配来自 payload 独立复制 + 缓冲增长，是可优化点（见 §6）。

### 3.2 消息分发（knet）

| 场景 | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| `MsgHandler.Execute`（直接执行，含中间件/分组查找 + 链构建） | 78.6 | 8 | 1 |
| `MsgHandler.SendMsgToTaskQueue`（入队 → worker 出队 → Execute 全路径） | 220 | 8 | 0 |

分发管线本身极快：220ns/op 且零分配。中间件链构建（`Execute` 内 global/groups/handlers 三个 slice 追加）是唯一固定分配（1 次、8B），可通过预分配优化。

### 3.3 连接写路径（knet，`BenchmarkConnectionSendMsg`，128B）

| 指标 | 值 |
|------|----|
| ns/op | 93 369（≈93μs，单条 128B 消息 server→client） |
| MB/s | 1.46（≈10.7k msg/s） |
| B/op / allocs/op | 512 / 8 |

写路径 = `codec.Pack`（~180ns）+ msgChan 队列 + writer goroutine 落盘 `conn.Write`（含每次 `SetWriteDeadline`）。**93μs 几乎全部是 Windows 环回 socket 系统调用延迟**，框架自身开销（Pack + channel）< 1μs。

### 3.4 缓冲池（kpool）

| 场景 | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| Get+Put 4KB 往返（热池） | 29.0 | 24 | 1 |
| Get+Put 16KB 往返（热池） | 38.6 | 24 | 1 |
| Get+Put 64KB 往返（热池） | 42.5 | 24 | 1 |
| Get 4KB（冷池，首触） | 1 161 | 4 123 | 2 |
| 直配 128KB（越级，不池化） | 17 185 | 131 072 | 1 |

热池往返 ~30–43ns；每 op 的 24B/1 alloc 是 **slice 装箱进 `any` 的接口头**（`sync.Pool` 以 `any` 存取），4KB 数据缓冲本身零拷贝复用。对比冷池首触 1.16μs、越级直配 17μs，**池化收益在冷启动/大缓冲场景明显**。

### 3.5 日志（klog）

| 场景 | ns/op | MB/s | B/op | allocs/op |
|------|-------|------|------|-----------|
| RingBuffer 写一条日志行 | 229 | 188 | 0 | 0 |
| RingBuffer.Lines(50)（读最近 N 行） | 14 050 | – | 11 520 | 3 |
| `Logger.Info`（slog，输出丢弃） | 784 | – | 0 | 0 |
| `Logger.Info` + AddSource（带 file:line） | 1 389 | – | 360 | 5 |

环形缓冲零分配写入（互斥锁 + 逐字节环形拷贝），可安全挂在热路径（如 MCP `get_logs` 后端）；`AddSource` 使每条日志 +5 次分配，生产开启需权衡。

### 3.6 配置（kconf）

| 场景 | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| `Default()` | 0.12 | 0 | 0 |
| `Load()`（默认值 + YAML + env 全链） | 54 763 | 12 004 | 104 |

`Load` 只在启动时调用一次（~55μs），运行期零开销；无需优化。

### 3.7 指标（kmetrics）

| 场景 | ns/op | B/op | allocs/op |
|------|-------|------|-----------|
| Counter.Inc（热路径埋点） | 6.3 | 0 | 0 |
| `Registry.Snapshot()`（Gather→dto 转换，21 个指标） | 36 233 | 50 754 | 289 |

埋点 6.3ns 零分配，对热路径（每消息、每字节计数）几乎无成本；Snapshot 仅在被 scrape / MCP 读取时执行。

## 4. 端到端吞吐（真实 TCP 环回）

### 4.1 单连接 echo（`BenchmarkEchoThroughput`，中位数，count=3）

| 配置 | ns/op（往返） | msg/s | MB/s（payload） |
|------|---------------|-------|-----------------|
| workers=1, 16B | 132 884 | 7 525 | 0.12 |
| workers=1, 128B | 143 744 | 6 956 | 0.89 |
| workers=1, 1KB | 140 658 | 7 110 | 7.28 |
| workers=4, 16B | 134 146 | 7 454 | 0.12 |
| workers=4, 128B | 138 699 | 7 210 | 0.92 |
| workers=4, 1KB | 139 718 | 7 158 | 7.33 |
| **裸 TCP 基线**（128B） | 62 419 | 16 021 | 2.05 |

关键结论：

1. **单连接是延迟受限，不是吞吐受限**：16B 与 1KB 的往返耗时几乎相同（~130–145μs），拷贝成本（Decode/Pack 见 §3.1，微秒级）在 RTT 面前可忽略；单连接 echo 稳定在 **7–7.5k msg/s**。
2. **workers=1 与 workers=4 无差异**：worker 按 ConnID 取模路由，单连接永远只进同一队列（保序设计，见 `docs/architecture.md`）；调大 worker 数提升的是**多连接**并行度。
3. **框架开销 ≈ 2.2× 裸 TCP 基线**：裸回显 62μs（无协议、无解析、单次 Read 整块回写），框架单条 128B 往返 ~139–144μs，多出的 ~75–80μs 为 decode + 分发 + Pack + channel 队列跳转 + 每写 `SetWriteDeadline`。这是"协议解析 + 并发模型 + 保序"的固定代价。

### 4.2 多连接并发 echo（`BenchmarkMultiConnEcho`，128B payload，workers=4，中位数）

| 连接数 | ns/op（每消息壁钟） | 聚合 msg/s | 聚合 MB/s |
|--------|---------------------|-----------|-----------|
| 1 | 117 640 | 8 502 | 1.09 |
| 8 | 25 840 | 38 700 | 4.95 |
| 32 | 18 620 | 53 705 | 6.87 |

**并发是吞吐的杠杆**：8 连接 → 4.5×，32 连接 → 6.3×（相对单连接）。增速递减源于共享 worker 池（4 worker）与环回调度/GC；在更多 worker 与多核 Linux 服务器上可继续线性扩展（20 线程仅用 4 worker）。32 连接 × 128B 聚合约 **54k msg/s / 6.9 MB/s**。

## 5. 每消息分配账本（端到端）

echo 路径单条消息（128B）**15 allocs / ~960B**，来源拆解：

| 环节 | 分配 |
|------|------|
| Decode：payload 独立复制 + Raw 复制 + msgs slice 增长 | ~4–5 |
| NewRequest（Request 结构 + 逃逸） | 1–2 |
| Execute：global/groups/handlers 三个 slice append + 绑链 | ~3–4 |
| SendMsg：NewMessage + Pack（bytes.Buffer + binary.Write） | ~4 |
| msgChan 传递 + 接口装箱 | ~1–2 |

> 对比：裸 TCP 回显 0 分配。分配的绝对量不大（~960B/条），但**高频消息场景下是 GC 压力的主要来源**，是未来优化的首选目标（见 §6）。

## 6. 优化建议（按收益排序，P7 之后可立项）

1. **Request 对象池**：`sync.Pool` 复用 `Request`（当前每消息 1–2 次分配），路由为无状态纯函数时收益直接。
2. **Decode 缓冲复用**：`Message.Data/Raw` 的独立复制改为**引用计数/租约缓冲**（如 `kpool` 分级缓冲 + 完成回调），可消掉每帧 ~3 次分配；代价是异步业务不得长期持有 payload（需要协议契约配合）。
3. **Pack 写路径**：`SetWriteDeadline` 每消息调用一次；对高频连接可缓存 deadline 或按批量设置。`bytes.Buffer` 换预分配缓冲（`make([]byte, 8+len)` 直接 binary 写）。
4. **中间件链预计算**：路由注册时把 global/groups/handlers 拼成最终链缓存，消掉 Execute 每消息的 3 次 slice 追加（当前 8B/1 alloc）。
5. **批量写**：writer 在队列空闲时批量取多条消息合并 `Write`（注意粘包语义由协议头保证），减少 syscall。
6. **跨平台复测**：Linux 服务器（epoll 调度、更低 syscall 延迟）上单连接 RTT 预计显著低于本机 Windows 数据。

## 7. 复现命令

```bash
# 微基准（每包）
go test ./knet/ ./kpool/ ./klog/ ./kconf/ ./kmetrics/ -bench "." -benchmem -benchtime=2s

# 端到端吞吐（含裸基线，-count=3 取中位数）
go test ./knet/ -bench "RawEchoBaseline|EchoThroughput|MultiConnEcho" -benchmem -benchtime=1s -count=3

# fuzz 冒烟（CI 用 5s 档）
go test ./knet/ -run '^$' -fuzz=FuzzTLVPackDecode -fuzztime=5s
go test ./klog/ -run '^$' -fuzz=FuzzRingBuffer -fuzztime=5s
go test ./kconf/ -run '^$' -fuzz=FuzzLoadYAML -fuzztime=5s
```

> Windows PowerShell 注意：`-bench "."` 的 `.` 需加引号，否则被当作别名解析导致基准不执行。
