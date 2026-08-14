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

## 如何验证 MCP 是否可用

### 方式一：跑项目自带测试（最快）

`go test ./kmcp/ -v` 已经覆盖了完整闭环：真实 Kinz server + 真实 TCP 连接 + mcp-go 客户端驱动，验证握手、10 工具、4 资源、stdio 传输、鉴权。全绿即 MCP 可用。

### 方式二：curl 直接调 streamable HTTP 端点（无需任何客户端）

1. 启动带 MCP 的服务：`go run ./examples/echo/server`（MCP 在 `http://127.0.0.1:9001/mcp`）
2. 握手（**保存响应头里的 `Mcp-Session-Id`**，后续请求都要带）：

```bash
curl -i -X POST http://127.0.0.1:9001/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl-test","version":"1.0"}}}'
```

3. 列出工具（把 `<SESSION>` 换成上一步拿到的值）：

```bash
curl -X POST http://127.0.0.1:9001/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: <SESSION>" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
```

4. 调用工具（示例 `server_info`；`tools/call` 的 `arguments` 传工具参数，无参工具传 `{}`）：

```bash
curl -X POST http://127.0.0.1:9001/mcp \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -H "Mcp-Session-Id: <SESSION>" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"server_info","arguments":{}}}'
```

5. 带真实连接验证操控闭环：另开一个 TCP 连接保持在线（如 `nc 127.0.0.1 8999`），然后 `list_connections` 应返回 `count: 1`；`send_to_connection` 传 `{"connID":1,"msgID":1,"data":"hi"}`，对端应收到 echo（echo demo 将 msgID 1 回显为 msgID 2）。

> 提示：Windows PowerShell 里请用 `curl.exe`（`curl` 是 `Invoke-WebRequest` 的别名），JSON 用**单引号**包裹且内部不要加反斜杠转义引号。

### 方式三：MCP Inspector（官方图形调试工具）

需要 Node.js：

```bash
# streamable HTTP 端点
npx @modelcontextprotocol/inspector http://127.0.0.1:9001/mcp

# 或 stdio 传输（Inspector 负责启动子进程并通过管道通信）
npx @modelcontextprotocol/inspector -- go run ./examples/mcp-stdio
```

浏览器打开 Inspector 面板：`Tools` 页逐个调用工具，`Resources` 页读取 `connections://`、`metrics://`、`config://`、`logs://`。

### 方式四：Claude Desktop（真实 AI 场景）

1. 编译 stdio 桥：`go build -o kinz-mcp-stdio ./examples/mcp-stdio`
2. 在 `claude_desktop_config.json` 的 `mcpServers` 指向该二进制（见上文示例）
3. 重启 Claude Desktop，直接问它："用 list_connections 看看当前有几个连接"——Claude 会调用工具并汇报结果

### 方式五：自己写个 mcp-go 客户端小程序

```go
package main

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

func main() {
	c, err := client.NewStreamableHttpClient("http://127.0.0.1:9001/mcp")
	if err != nil {
		panic(err)
	}
	if _, err := c.Initialize(context.Background(), mcp.InitializeRequest{}); err != nil {
		panic(err)
	}
	res, err := c.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "server_info", Arguments: map[string]any{}},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(res.Content)
}
```

### 验证清单（期望结果）

| 调用 | 期望 |
|------|------|
| `tools/list` | 返回 10 个工具 |
| `server_info` | 含 `name`/`version`/`address`/`uptimeSeconds` |
| `list_connections` | `count` 与在线连接数一致；有连接时含 `id`/`remote`/`local` |
| `send_to_connection` | 对端实际收到该消息 |
| `broadcast` | 返回 `sent` 计数 = 在线连接数，所有对端收到 |
| `get_metrics` | 含 `kinz_conns_total` 等计数 |
| `get_config` | 含 `name`、`port` 等生效配置 |
| `get_logs` | 返回环形缓冲最近日志行 |
| `close_connection` | 对端收到 EOF（连接断开） |
| `shutdown_server` | 服务器优雅停机（谨慎） |
| `resources/read`（`metrics://` 等） | 返回对应内容 |

## 安全

- 默认无鉴权（面向开发/内网运维）；生产环境务必通过 `WithAuth` 加权限控制（至少拦截 `close_connection`、`shutdown_server`）。
- `shutdown_server` 会真正关闭服务器——谨慎授权。

## 设计说明

- `kmcp` 是**独立适配器**：只依赖 `kiface`/`kconf`/`klog`/`kmetrics` + mcp-go，不依赖 `knet`（应用负责接线），因此无循环依赖、可独立测试。
- MCP SDK 依赖仅在 import kmcp 时链接——这就是"可选开启"的代价边界。

