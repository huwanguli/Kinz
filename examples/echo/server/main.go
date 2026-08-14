// Command server runs the Kinz echo demo server.
//
// It replies to every ping (msgID 1) with a pong (msgID 2), logs each message
// through global middleware (before + after, onion model), enables heartbeat
// checking (10s interval), shuts down gracefully on Ctrl+C / SIGTERM, and
// exposes two management endpoints:
//
//   - Prometheus metrics: http://127.0.0.1:9000/metrics (AttachMetrics)
//   - MCP server for AI tools: tcp://127.0.0.1:9001 (kmcp, JSON-RPC over TCP)
//
// Run:  go run ./examples/echo/server
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
	"kinz/kmcp"
	"kinz/knet"
)

func main() {
	cfg := kconf.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = 8999

	// Ring-buffer log backend so the MCP get_logs tool can read recent lines.
	ring := klog.NewRingBuffer(256 * 1024)
	klog.SetDefault(klog.New(klog.Options{Output: io.MultiWriter(os.Stdout, ring)}))

	s := knet.NewServer(knet.WithConfig(cfg))

	// Global middleware 1 (before-only): log every message, then continue.
	if _, err := s.Use(func(req kiface.IRequest) {
		klog.L().Info("message received",
			"msgID", req.GetMsgID(),
			"remote", req.GetConnection().GetRemoteAddr().String())
		req.RouterSlicesNext()
	}); err != nil {
		fatal("use middleware", err)
	}

	// Global middleware 2 (before + after, onion model): time the whole chain.
	if _, err := s.Use(func(req kiface.IRequest) {
		start := time.Now()
		req.RouterSlicesNext()
		klog.L().Info("handled", "msgID", req.GetMsgID(), "elapsed", time.Since(start).String())
	}); err != nil {
		fatal("use timing middleware", err)
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

	// Management endpoints (best-effort; failures are logged, not fatal).
	if err := s.AttachMetrics("127.0.0.1:9000"); err != nil {
		klog.L().Warn("metrics endpoint failed", "err", err)
	}
	mcp := kmcp.NewServer(s, kmcp.WithConfig(cfg), kmcp.WithLogRing(ring))
	go func() {
		if err := mcp.ListenAndServe("127.0.0.1:9001"); err != nil {
			klog.L().Warn("mcp endpoint failed", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	klog.L().Info("server starting",
		"name", s.Name(),
		"addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		"metrics", "http://127.0.0.1:9000/metrics",
		"mcp", "tcp://127.0.0.1:9001")
	if err := s.Serve(ctx); err != nil {
		fatal("serve", err)
	}
	klog.L().Info("server stopped")
}

func fatal(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
