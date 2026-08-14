// Command mcp-stdio is a stdio MCP bridge: it runs a Kinz server (configured
// via environment) in-process and exposes it to AI tools over the MCP stdio
// transport. Point Claude Desktop's mcpServers entry at this binary.
//
// Environment: KINZ_HOST (default 127.0.0.1), KINZ_PORT (default 8999).
// Logs go to stderr (stdout is reserved for the JSON-RPC transport).
//
// Run:  go run ./examples/mcp-stdio
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
	"kinz/kmcp"
	"kinz/knet"
)

func main() {
	cfg := kconf.Default()
	cfg.Host = envOr("KINZ_HOST", "127.0.0.1")
	cfg.Port = envInt("KINZ_PORT", 8999)

	// Logs to stderr (stdout is the MCP transport); keep a ring for get_logs.
	ring := klog.NewRingBuffer(256 * 1024)
	klog.SetDefault(klog.New(klog.Options{Output: io.MultiWriter(os.Stderr, ring)}))

	s := knet.NewServer(knet.WithConfig(cfg))

	// A sample business route: broadcast every msgID 1 message as msgID 2.
	if _, err := s.AddRouterSlices(1, func(req kiface.IRequest) {
		s.GetConnMgr().Range(func(_ uint64, c kiface.IConnection) bool {
			_ = c.SendMsg(2, req.GetData())
			return true
		})
	}); err != nil {
		fatal("register router", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { _ = s.Serve(ctx) }()

	// MCP over stdio (Claude Desktop launches this binary and speaks to it).
	mcp := kmcp.NewServer(s, kmcp.WithConfig(cfg), kmcp.WithLogRing(ring))
	klog.L().Info("mcp-stdio bridge ready", "addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err := mcp.ServeStdio(); err != nil {
		fatal("mcp-stdio", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func fatal(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
