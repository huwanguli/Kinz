// Command client is a Kinz chatroom demo client: it joins, sends one message,
// and prints every broadcast it receives for a few seconds.
//
// Run:  go run ./examples/chatroom/client [name]   (default user1)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"kinz/kiface"
	"kinz/knet"
)

func main() {
	name := "user1"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	c := knet.NewClient("127.0.0.1", 8999, knet.WithReconnect(500*time.Millisecond, 3*time.Second, 2))

	received := make(chan string, 32)
	if _, err := c.AddRouterSlices(2, func(req kiface.IRequest) {
		received <- string(req.GetData())
	}); err != nil {
		fatal("register router", err)
	}
	if err := c.Start(); err != nil {
		fatal("connect", err)
	}
	defer c.Stop()

	msg := fmt.Sprintf("%s: hello everyone", name)
	if err := c.Conn().SendMsg(1, []byte(msg)); err != nil {
		fatal("send", err)
	}
	fmt.Println("sent:", msg)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		select {
		case m := <-received:
			fmt.Println("recv:", m)
		case <-ctx.Done():
			fmt.Println("done")
			return
		}
	}
}

func fatal(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
