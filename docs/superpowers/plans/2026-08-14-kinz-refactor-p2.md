# Kinz 重构实施计划（P2：核心实现重写）

日期：2026-08-14
模式：连续执行（用户指示"直接一直做直到 P2 完成"，无逐 Task 暂停）
来源：`docs/superpowers/specs/2026-08-14-zinx-production-refactor-design.md` §7 P2

## 目标

Server 生命周期、Connection、消息管线、RouterSlices/中间件、心跳、满连接拒绝、kconf（YAML）全部真实现；`examples/ping` 用新 API 跑通；集成测试覆盖心跳超时/满连接拒绝/优雅停机/粘包；退出标准见下。

## 关键设计决策（含对 P1 接口的修订，均需在 P2 落地）

0. **P2 设计修订（用户反馈）**：
   - **codec 合并**（覆盖原决策 1–4）：`IDecoder`（帧解码）与 `IDataPack`（消息解析）两个耦合接口冗余 → **合并为单一 `kiface.ICodec`**（`Decode` + `Pack` + `Clone`），默认实现 `knet.TLVPack`（payload 独立复制修复数据竞争）；删除 `FrameDecoder`、`LengthField`；`IServer`/`IClient` 统一 `SetCodec/GetCodec`。
   - **删除拦截器链**：`IInterceptor` 责任链与 `Use`/`Group` 中间件功能重叠且被完全覆盖 → 删除 `kinterceptor` 包、`IInterceptor/IChain/IcReq/IcResp`、`IRequest.GetResponse/SetResponse`、`AddInterceptor/SetHeadInterceptor`；`MsgHandler.Execute` 只做中间件链分发。
   - **删除死配置字段**：`kconf.Config` 的 `HeartbeatInterval/HeartbeatTimeout/ReadIdleTimeout` 框架零读取（心跳由 `StartHeartBeat` 显式配置）→ 删除。
1. **kiface 修订**（P2 接口修订，随 P2 提交）：
   - `IServer.Address() net.Addr`（Port=0 时测试/运维需要真实地址）
   - `HeartBeatOption.Timeout time.Duration`（超时判定，默认 3×interval）
   - 新增常量 `ServerFullMsgID uint32 = 0xFFFFFFFE`（满连接拒绝时发送的消息 ID）
2. **读路径**：不用 bufio，直接 `conn.Read` 读入 **kpool 池化缓冲**（每连接 Get(4096)，关闭时 Put）——codec 自带帧重组，bufio 冗余。
5. **kpool**：4K/16K/64K 三档 `sync.Pool`；`Get(size)` 返回 ≥size 档位缓冲，超 64K 直接分配；`Put` 校验档位，不匹配静默丢弃；Put 时首字节写哨兵 0xAA（误用检测）。
6. **kconf**：`Config` 全字段 + `Duration` 自定义类型（YAML 支持 "10s" 字符串与纳秒整数）；加载链 默认→`conf/kinz.yaml`（缺失不 panic，非法报错）→`KINZ_*` 环境变量（非法报错）；`Load(path) (*Config, error)`。
7. **Connection**：`stopOnce sync.Once` 幂等清理（hooks/hb 停止/socket 关闭/done 关闭/ConnMgr 移除），Reader 的 defer 只调 `stopOnce`（不等待自身），外部 `Stop()` = stopOnce + `wg.Wait`；`msgChan` 缓冲 `WriteQueueSize`（默认 256）**永不 close**，Writer 靠 done 退出；`SendMsg` 三路 select（msgChan/done/WriteTimeout）；`lastActivity` 原子时间戳，任何消息刷新，`IsAlive(timeout)` 判定。
8. **MsgHandler**：`classicApis`（IRouter）+ `apis`（RouterHandler 切片）+ `globalHandlers` + `groupRanges`；`routerSlices`/`groupRouterSlices` 具体类型实现 P1 接口；`Execute` = 中间件链分发（含 recover）；Worker 池 ctx 取消 + 排空退出；池大小为 0 时直启 goroutine；重复注册返回 `ErrMsgIDRegistered`。
9. **心跳接线**：`StartHeartBeat(interval)`/`SetHeartBeatWithOption` 生成模板存入 Server；`Connection.Start()` 克隆模板→BindConn→Start；默认路由 `HeartBeatDefaultHandle` 自动注册（忽略重复注册错误）；修复旧 `if msgFunc == nil` 判断写反的 bug。
10. **满连接拒绝**：accept 循环 `connMgr.Add` 返回 `ErrServerFull` → `rejectConn`：打包发送 `ServerFullMsgID` 消息（写超时保护）→ 关闭。
11. **Server 生命周期**：`Run(ctx)` 启动 listener/Worker 池/accept 循环，阻塞至 ctx 取消返回 nil；`Shutdown(ctx)` = 关 listener → `ClearConn` 排空（ctx 超时）→ `StopWorkerPool(ctx)`，幂等；`Serve(ctx)` = Run + Shutdown 组合；`connID` 原子递增。

## 任务清单（每任务含测试，随实现同批提交）

| # | 任务 | 文件 | 测试 |
|---|------|------|------|
| T1 | go.mod + kiface 修订 | `go.mod`、`kiface/idecoder.go`、`kiface/iserver.go`、`kiface/iheartbeat.go`、`kiface/errors.go`(常量) | 编译期断言随 T5 更新 |
| T2 | kconf | `kconf/config.go`、`kconf/config_test.go` | 默认/缺文件/YAML/非法 YAML/env 覆盖 |
| T3 | kpool | `kpool/pool.go`、`kpool/pool_test.go` | 档位获取/复用/误尺寸丢弃/大缓冲直配 |
| T4 | codec 合并：TLVPack(ICodec) + 删 FrameDecoder/LengthField | `kiface/icodec.go`、`knet/datapack.go` | 往返/半包/粘包/超长/大端/Clone/payload 独立 |
| T4b | 删拦截器链（kinterceptor 包、IInterceptor、GetResponse/SetResponse）+ 删死配置字段 | `kiface/*`、`knet/*`、`kconf/*` | 既有测试全绿 + 覆盖率门禁 |
| T5 | Request | `knet/request.go` | 上下文/Copy/Abort 语义（随 T6 测试） |
| T6 | RouterSlices+MsgHandler | `knet/routerSlices.go`、`knet/msgHandler.go` | 重复注册/中间件顺序/Abort/Group 越界/panic 恢复 |
| T7 | HeartBeatChecker | `knet/heartbeat.go` | 存活/超时回调/默认消息/Set 函数非 nil 判断 |
| T8 | Connection | `knet/connection.go` | 属性/关闭后 SendMsg 返回 ErrConnClosed（真 TCP） |
| T9 | Server | `knet/server.go`、`knet/options.go`、`knet/connmanager.go`(核对) | 集成测试见 T10 |
| T10 | 集成测试 | `knet/server_integration_test.go` | echo/粘包/满连接拒绝/心跳超时/优雅停机 |
| T11 | examples/ping + 验证 | `examples/ping/main.go`、AGENT.md | build 冒烟 + 覆盖率门禁 |

## 覆盖率门禁（P2 退出标准）

- `go build ./...`、`go vet ./...`、`go test ./...` 全绿（`-race` 因本机无 C 工具链，环境可用时执行）
- 覆盖率：kconf ≥ 80%、kpool ≥ 80%、knet ≥ 55%（集成测试驱动）
- `examples/ping` 可编译
- 无 `panic("implement me")`、无裸 panic 表达协议错误、无 `fmt.Printf` 日志（全部走 klog）

## 提交节奏

T1–T4 各自独立提交；T5–T9 可合并为 1–2 个提交；T10 集成测试独立提交；T11 收尾提交。每个提交可独立编译（除中间接口修订期短暂 broken，尽量保持绿）。
