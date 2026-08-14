# MCP Server（kmcp）

Kinz 通过 `kmcp` 包把运行中的服务器暴露给 AI 工具（Claude Desktop、IDE、任意 MCP 客户端），实现"AI 开发友好"的运行时闭环。

> **kmcp 是可选开启项（opt-in）**：核心框架从不 import 它；只有你的应用显式 import `kinz/kmcp` 时才会链接（并引入 MCP SDK 依赖）。不需要 MCP 的应用零影响。

## 实现

- 基于 **mark3labs/mcp-go**（MCP 官方协议规范完整实现），工具/资源注册在 SDK 之上。
- 传输：**stdio**（Claude Desktop 约定）与 **streamable HTTP**（远程客户端标准传输）。
- 能力：`initialize` 握手、`tools/list`、`tools/call`、`resources/list`、`resources/read`、`ping`、`notifications`。

## 接入方式

```go
import "kinz/kmcp"

s := knet.NewServer(...)

// streamable HTTP 端点（标准远程 MCP 传输；默认路径 /mcp）
mcp := kmcp.NewServer(s, kmcp.WithConfig(cfg), kmcp.WithLogRing(ring))
go func() { _ = mcp.ServeHTTP("127.0.0.1:9001") }()

// 挂到自己的 HTTP mux
mux.Handle("/mcp", mcp.Handler())

// stdio 端点（给 Claude Desktop 指向可执行文件）
mcp.ServeStdio()
```

选项：
- `kmcp.WithConfig(cfg *kconf.Config)`：get_config 工具/资源返回的配置。
- `kmcp.WithLogRing(ring *klog.RingBuffer)`：get_logs 工具/资源读取的日志环形缓冲（配合 `klog.NewRingBuffer` + `klog.SetDefault` 使用）。
- `kmcp.WithAuth(f AuthFunc)`：鉴权回调，`f(method)` 返回 error 即拒绝该调用（用于 tools/call 与 resources/read）。
- `kmcp.WithVersion(v string)`：自定义 server 版本。

## 工具（Tools）

| 工具 | 参数 | 说明 |
|------|------|------|
| `server_info` | — | 名称、版本、监听地址、运行时长 |
| `list_connections` | — | 在线连接列表（id/remote/local） |
| `get_connection` | `connID` | 单连接详情 |
| `send_to_connection` | `connID, msgID, data` | 向指定连接发送消息 |
| `broadcast` | `msgID, data` | 向所有连接广播，返回 `sent` 计数 |
| `close_connection` | `connID` | 优雅关闭指定连接 |
| `get_metrics` | — | 全部指标（counters/gauges/histograms） |
| `get_config` | — | 当前生效配置 |
| `get_logs` | `lines`(可选,默认50) | 环形缓冲最近日志 |
| `shutdown_server` | — | 触发优雅停机 |

## 资源（Resources）

| URI | 内容 |
|-----|------|
| `connections://` | 在线连接 |
| `metrics://` | 指标快照 |
| `config://` | 生效配置 |
| `logs://` | 最近日志 |

## 示例（Claude Desktop stdio 配置）

`claude_desktop_config.json`：

```json
{
  "mcpServers": {
    "kinz": {
      "command": "kinz-mcp-stdio",
      "args": []
    }
  }
}
```

任意 MCP 客户端连接 streamable HTTP 端点后即可调用工具（如 `list_connections`、`send_to_connection`）。

## 安全

- 默认无鉴权（面向开发/内网运维）；生产环境务必通过 `WithAuth` 加权限控制（至少拦截 `close_connection`、`shutdown_server`）。
- `shutdown_server` 会真正关闭服务器——谨慎授权。

## 设计说明

- `kmcp` 是**独立适配器**：只依赖 `kiface`/`kconf`/`klog`/`kmetrics` + mcp-go，不依赖 `knet`（应用负责接线），因此无循环依赖、可独立测试。
- MCP SDK 依赖仅在 import kmcp 时链接——这就是"可选开启"的代价边界。

