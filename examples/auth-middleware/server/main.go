// Command server runs a Kinz auth-middleware demo: a Use middleware requires
// every message (except the auth message) to carry an authenticated connection
// property; unauthenticated messages are aborted with a "unauthorized" notice.
//
// Protocol: msgID 0 = auth (token "secret"), msgID 1 = ping (auth required),
// msgID 2 = pong, msgID 3 = notice (authorized / unauthorized / bad token).
//
// Run:  go run ./examples/auth-middleware/server
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

const (
	msgAuth   = uint32(0) // auth message
	msgPing   = uint32(1) // business message (auth required)
	msgPong   = uint32(2) // echo reply
	msgNotice = uint32(3) // auth notice
)

const token = "secret"

func main() {
	cfg := kconf.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = 8999

	s := knet.NewServer(knet.WithConfig(cfg))

	// Auth middleware: everything except the auth message must be authed.
	if _, err := s.Use(func(req kiface.IRequest) {
		if req.GetMsgID() == msgAuth {
			req.RouterSlicesNext()
			return
		}
		if authed, _ := req.GetConnection().GetProperty("authed"); authed == true {
			req.RouterSlicesNext()
			return
		}
		_ = req.GetConnection().SendMsg(msgNotice, []byte("unauthorized"))
		req.Abort() // stop the chain
	}); err != nil {
		fatal("use middleware", err)
	}

	// Auth handler: validate the token and mark the connection.
	if _, err := s.AddRouterSlices(msgAuth, func(req kiface.IRequest) {
		if string(req.GetData()) == token {
			req.GetConnection().SetProperty("authed", true)
			_ = req.GetConnection().SendMsg(msgNotice, []byte("authorized"))
		} else {
			_ = req.GetConnection().SendMsg(msgNotice, []byte("bad token"))
		}
	}); err != nil {
		fatal("register auth router", err)
	}

	// Business: echo (only reachable when authenticated).
	if _, err := s.AddRouterSlices(msgPing, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(msgPong, req.GetData())
	}); err != nil {
		fatal("register ping router", err)
	}

	s.StartHeartBeat(10 * time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	klog.L().Info("auth-middleware starting", "addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))
	if err := s.Serve(ctx); err != nil {
		fatal("serve", err)
	}
}

func fatal(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
