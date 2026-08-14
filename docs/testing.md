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

- 当前实际：kmetrics 89.6% / klog 96.3% / knet 73.7% / kconf 86.2% / kpool 100% / kmcp 65.8%
- P6 目标：核心包（knet/kconf/kmetrics/klog/kpool）≥ 70%（knet 当前已达标）

## 模糊测试

P6 将补充 `TLVPack.Decode` 的 fuzz 测试：

```go
func FuzzTLVPackDecode(f *testing.F) {
	f.Add([]byte{0x02, 0, 0, 0, 0x01, 0, 0, 0, 'h', 'i'})
	f.Fuzz(func(t *testing.T, data []byte) {
		codec := NewTLVPack()
		_, _ = codec.Decode(data) // 必须不 panic、不崩溃
	})
}
```

## 基准测试

P6 将补充：TLVPack Pack/Decode、单连接吞吐、多连接并发、缓冲池命中率的 benchmark。

## 集成测试如何写

- 服务端用 `127.0.0.1:0`（随机端口）+ `srv.Address()` 取真实地址，避免端口冲突。
- 客户端可以是原始 TCP + `knet.NewTLVPack()`（测试编解码与协议），或 `knet.Client`（测试框架客户端）。
- 配置一律从 `kconf.Default()` 派生再覆盖，避免结构体字面量把默认字段清零。
