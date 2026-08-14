// Command client is the Kinz echo demo client, built on the framework's own
// knet.Client (dial, auto-reconnect, routing, heartbeat).
//
// It connects, sends three pings (msgID 1) in a burst, expects three pongs
// (msgID 2) with matching payloads, then exits.
//
// Run:  go run ./examples/echo/client [addr]   (default 127.0.0.1:8999)
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"kinz/kiface"
	"kinz/knet"
)

func main() {
	host, port := "127.0.0.1", 8999
	if len(os.Args) > 1 {
		h, p, err := net.SplitHostPort(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad address:", err)
			os.Exit(1)
		}
		port, err = strconv.Atoi(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad port:", err)
			os.Exit(1)
		}
		host = h
	}

	client := knet.NewClient(host, port,
		knet.WithReconnect(500*time.Millisecond, 3*time.Second, 2))

	responses := make(chan string, 3)
	if _, err := client.AddRouterSlices(2, func(req kiface.IRequest) {
		responses <- string(req.GetData())
	}); err != nil {
		fatal("register router", err)
	}

	if err := client.Start(); err != nil {
		fatal("connect", err)
	}
	defer client.Stop()
	fmt.Printf("connected to %s:%d\n", host, port)

	for i := 1; i <= 3; i++ {
		if err := client.Conn().SendMsg(1, []byte(fmt.Sprintf("ping-%d", i))); err != nil {
			fatal("send", err)
		}
	}

	for i := 1; i <= 3; i++ {
		want := fmt.Sprintf("ping-%d", i)
		select {
		case data := <-responses:
			if data != want {
				fatalf("unexpected pong %q, want %q", data, want)
			}
			fmt.Printf("pong %d/%d: %q\n", i, 3, data)
		case <-time.After(5 * time.Second):
			fatal("timeout waiting for pong", nil)
		}
	}
	fmt.Println("echo OK")
}

func fatal(what string, err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, what+":", err)
	} else {
		fmt.Fprintln(os.Stderr, what)
	}
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
