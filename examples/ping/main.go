// Command ping is a minimal Kinz server demo: it echoes a "pong" message for
// every ping (msgID 1) and enables heartbeat checking.
//
// Run: go run ./examples/ping
// Then speak TLV with any client (e.g. use knet.DataPack in a test program).
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kinz/kiface"
	"kinz/klog"
	"kinz/knet"
)

func main() {
	s := knet.NewServer()
	if _, err := s.AddRouterSlices(1, func(req kiface.IRequest) {
		klog.L().Info("ping received", "remote", req.GetConnection().GetRemoteAddr().String(), "data", string(req.GetData()))
		if err := req.GetConnection().SendMsg(2, []byte("pong")); err != nil {
			klog.L().Warn("send pong failed", "err", err)
		}
	}); err != nil {
		fmt.Fprintln(os.Stderr, "register router:", err)
		os.Exit(1)
	}

	s.StartHeartBeat(10 * time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	klog.L().Info("server starting", "name", s.Name())
	if err := s.Serve(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "serve:", err)
		os.Exit(1)
	}
	klog.L().Info("server stopped")
}
