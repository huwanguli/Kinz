// Command server runs the Kinz echo demo server.
//
// It replies to every ping (msgID 1) with a pong (msgID 2), logs each message
// through a global middleware, enables heartbeat checking (10s interval), and
// shuts down gracefully on Ctrl+C / SIGTERM.
//
// Run:  go run ./examples/echo/server
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

	// Global middleware: log every message, then continue the chain.
	if _, err := s.Use(func(req kiface.IRequest) {
		klog.L().Info("message received",
			"msgID", req.GetMsgID(),
			"remote", req.GetConnection().GetRemoteAddr().String())
		req.RouterSlicesNext()
	}); err != nil {
		fatal("use middleware", err)
	}

	// Echo handler: ping (msgID 1) -> pong (msgID 2) with the same payload.
	if _, err := s.AddRouterSlices(1, func(req kiface.IRequest) {
		klog.L().Info("echo", "data", string(req.GetData()))
		if err := req.GetConnection().SendMsg(2, req.GetData()); err != nil {
			klog.L().Warn("send pong failed", "err", err)
		}
	}); err != nil {
		fatal("register router", err)
	}

	s.StartHeartBeat(10 * time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	klog.L().Info("server starting",
		"name", s.Name(),
		"addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err := s.Serve(ctx); err != nil {
		fatal("serve", err)
	}
	klog.L().Info("server stopped")
}

func fatal(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
