# Kinz 配置

配置加载链（优先级从低到高）：**内置默认值 → `conf/kinz.yaml`（可选，缺失不报错）→ `KINZ_*` 环境变量 → 代码 Option 覆盖**。

```go
import "kinz/kconf"
import "kinz/knet"

cfg := kconf.Default()          // 内置默认
cfg, _ = kconf.Load("conf/kinz.yaml") // 文件 + env
s := knet.NewServer(knet.WithConfig(cfg))
```

## 字段一览

| 字段 | 默认 | 说明 |
|------|------|------|
| `Name` | `KinzServer` | 服务器名 |
| `Host` | `0.0.0.0` | 监听地址 |
| `Port` | `8999` | 监听端口（`0` = 随机端口，用 `srv.Address()` 获取） |
| `MaxConn` | `1024` | 最大连接数（超限发送 `ServerFullMsgID` 后关闭） |
| `MaxPacketSize` | `4096` | 单包载荷上限（超限 fail-fast） |
| `WorkerPoolSize` | `10` | Worker 池大小（`0` = 每消息一个 goroutine，无序） |
| `MaxWorkerTaskLen` | `1024` | 每个 Worker 队列长度（满则背压阻塞） |
| `WriteQueueSize` | `256` | 每连接写缓冲（消息数），满则 `SendMsg` 按 `WriteTimeout` 超时 |
| `WriteTimeout` | `5s` | 写队列满/写 socket 的超时 |

> 心跳**不在此配置**：由 `srv.StartHeartBeat(interval)` / `SetHeartBeatWithOption` 显式配置（避免配置与实际行为漂移）。

## YAML 示例（`conf/kinz.yaml`）

```yaml
Name: GameServer
Host: 0.0.0.0
Port: 9000
MaxConn: 2000
MaxPacketSize: 16384
WorkerPoolSize: 16
MaxWorkerTaskLen: 2048
WriteQueueSize: 512
WriteTimeout: 3s
```

- 缺失文件不报错（用默认值）；非法 YAML / 非法 duration 返回 error。
- duration 支持字符串（`"3s"`/`"500ms"`）或纳秒整数。

## 环境变量（`KINZ_*`）

| 变量 | 对应字段 |
|------|---------|
| `KINZ_NAME` | Name |
| `KINZ_HOST` | Host |
| `KINZ_PORT` | Port |
| `KINZ_MAXCONN` | MaxConn |
| `KINZ_MAXPACKETSIZE` | MaxPacketSize |
| `KINZ_WORKERPOOLSIZE` | WorkerPoolSize |
| `KINZ_MAXWORKERTASKLEN` | MaxWorkerTaskLen |
| `KINZ_WRITEQUEUESIZE` | WriteQueueSize |
| `KINZ_WRITETIMEOUT` | WriteTimeout（Go duration 字符串） |

## 代码 Option（最高优先级）

```go
s := knet.NewServer(
    knet.WithConfig(cfg),
    knet.WithMaxConn(5000),   // 覆盖 cfg.MaxConn
    knet.WithName("MyServer"),
    knet.WithTLS(tlsCfg),     // 启用 TLS
)
```

Client 侧 `knet.NewClient(host, port, knet.WithReconnect(initial, max, mult), knet.WithTLSClient(cfg))`。
