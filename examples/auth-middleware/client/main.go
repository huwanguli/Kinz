// Command client exercises the auth-middleware demo: it first sends a ping
// without auth (expects "unauthorized"), then authenticates with the token,
// then sends a ping again (expects the echo).
//
// Run:  go run ./examples/auth-middleware/client
package main

import (
	"fmt"
	"os"
	"time"

	"kinz/kiface"
	"kinz/knet"
)

const (
	msgAuth   = uint32(0)
	msgPing   = uint32(1)
	msgPong   = uint32(2)
	msgNotice = uint32(3)
)

func main() {
	c := knet.NewClient("127.0.0.1", 8999, knet.WithReconnect(500*time.Millisecond, 3*time.Second, 2))

	responses := make(chan string, 8)
	_, _ = c.AddRouterSlices(msgPong, func(req kiface.IRequest) {
		responses <- "pong: " + string(req.GetData())
	})
	_, _ = c.AddRouterSlices(msgNotice, func(req kiface.IRequest) {
		responses <- "notice: " + string(req.GetData())
	})

	if err := c.Start(); err != nil {
		fatal("connect", err)
	}
	defer c.Stop()

	// 1. unauthenticated ping -> unauthorized notice
	if err := c.Conn().SendMsg(msgPing, []byte("hi")); err != nil {
		fatal("send", err)
	}
	fmt.Println("1:", waitFor(responses))

	// 2. authenticate
	if err := c.Conn().SendMsg(msgAuth, []byte("secret")); err != nil {
		fatal("send", err)
	}
	fmt.Println("2:", waitFor(responses))

	// 3. authenticated ping -> echo
	if err := c.Conn().SendMsg(msgPing, []byte("hi")); err != nil {
		fatal("send", err)
	}
	fmt.Println("3:", waitFor(responses))

	fmt.Println("auth demo OK")
}

func waitFor(ch <-chan string) string {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	select {
	case s := <-ch:
		return s
	case <-timer.C:
		return "<timeout>"
	}
}

func fatal(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
