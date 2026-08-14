// Command server runs a Kinz chatroom demo: every chat message (msgID 1) is
// broadcast to all live connections (msgID 2), with join/leave hooks, heartbeat
// checking, and graceful shutdown.
//
// Run:  go run ./examples/chatroom/server
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
	"kinz/knet"
)

func main() {
	cfg := kconf.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = 8999

	s := knet.NewServer(knet.WithConfig(cfg))

	// Join/leave hooks (observe online count via ConnManager).
	s.SetOnConnStart(func(conn kiface.IConnection) {
		klog.L().Info("user joined", "remote", conn.GetRemoteAddr().String(), "online", s.GetConnMgr().Len())
	})
	s.SetOnConnStop(func(conn kiface.IConnection) {
		klog.L().Info("user left", "remote", conn.GetRemoteAddr().String(), "online", s.GetConnMgr().Len())
	})

	// Chat message (msgID 1) -> broadcast to everyone (msgID 2).
	if _, err := s.AddRouterSlices(1, func(req kiface.IRequest) {
		klog.L().Info("chat", "from", req.GetConnection().GetRemoteAddr().String(), "data", string(req.GetData()))
		s.GetConnMgr().Range(func(_ uint64, c kiface.IConnection) bool {
			_ = c.SendMsg(2, req.GetData())
			return true
		})
	}); err != nil {
		fatal("register router", err)
	}

	s.StartHeartBeat(10 * time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	klog.L().Info("chatroom starting", "addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err := s.Serve(ctx); err != nil {
		fatal("serve", err)
	}
	klog.L().Info("chatroom stopped")
}

func fatal(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
