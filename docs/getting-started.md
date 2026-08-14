# Kinz 快速开始

## 最小服务端（echo）

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"kinz/kiface"
	"kinz/klog"
	"kinz/knet"
)

func main() {
	s := knet.NewServer()

	// ping (msgID 1) -> pong (msgID 2)，回显载荷
	if _, err := s.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		panic(err)
	}

	s.StartHeartBeat(10 * time.Second) // 心跳默认开

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	klog.L().Info("starting")
	if err := s.Serve(ctx); err != nil { // 优雅停机
		panic(err)
	}
}
```

## 最小客户端

```go
c := knet.NewClient("127.0.0.1", 8999, knet.WithReconnect(500*time.Millisecond, 3*time.Second, 2))

pongs := make(chan string, 1)
_, _ = c.AddRouterSlices(2, func(req kiface.IRequest) { pongs <- string(req.GetData()) })

if err := c.Start(); err != nil { panic(err) }   // dial；之后断线自动重连
_ = c.Conn().SendMsg(1, []byte("hello"))          // 发 ping
got := <-pongs                                     // 收 pong
c.Stop()
```

## 中间件（洋葱模型：before/after）

```go
// 全局计时中间件：RouterSlicesNext() 之后的代码在业务处理完后执行
if _, err := s.Use(func(req kiface.IRequest) {
	start := time.Now()
	req.RouterSlicesNext()
	klog.L().Info("handled", "msgID", req.GetMsgID(), "elapsed", time.Since(start).String())
}); err != nil {
	panic(err)
}
```

- `req.RouterSlicesNext()` 续链（必须显式调用）；`req.Abort()` 终止后续 handler。
- `req.SetMessage(...)` 替换消息（解密场景）；`req.Set/Get` 请求级上下文。

## 常用 API 速查

| 目的 | API |
|------|-----|
| 建服务器 | `knet.NewServer(opts...)` |
| 注册路由 | `s.AddRouterSlices(msgID, handlers...)` |
| 全局/区间中间件 | `s.Use(handlers...)` / `s.Group(start, end, handlers...)` |
| 生命周期 | `s.Serve(ctx)` / `s.Run(ctx)` / `s.Shutdown(ctx)` |
| 心跳 | `s.StartHeartBeat(interval)` |
| 指标端点 | `s.AttachMetrics(":9000")`（`/metrics`） |
| MCP（可选） | `kmcp.NewServer(s, opts...).ServeHTTP(":9001")` |
| 建客户端 | `knet.NewClient(host, port, opts...)` |
| 换协议 | 实现 `kiface.ICodec`，`s.SetCodec(codec)` |
| 连接属性 | `conn.SetProperty/GetProperty/RemoveProperty` |
| 优雅停机 | Ctrl+C（demo 已接信号）或 `s.Shutdown(ctx)` |

## 运行示例

```bash
go run ./examples/echo/server        # echo 服务端（8999 + metrics :9000 + MCP :9001/mcp）
go run ./examples/echo/client        # echo 客户端（3 次 ping-pong）
go run ./examples/chatroom/server    # 聊天室广播
go run ./examples/chatroom/client    # 聊天室客户端
go run ./examples/auth-middleware/server  # 鉴权中间件演示
```

更多：`docs/protocol.md`（线协议）、`docs/configuration.md`（配置）、`docs/mcp.md`（MCP）。
