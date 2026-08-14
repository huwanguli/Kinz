// Command client is a raw TLV client for the Kinz echo demo server.
//
// It connects, sends three pings (msgID 1) in a burst, expects three pongs
// (msgID 2) with matching payloads, and exits. It also skips heartbeat frames
// (msgID 99999) that the server sends periodically.
//
// Run:  go run ./examples/echo/client [addr]   (default 127.0.0.1:8999)
package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"kinz/kiface"
	"kinz/knet"
)

func main() {
	addr := "127.0.0.1:8999"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		fatal("dial", err)
	}
	defer conn.Close()
	fmt.Printf("connected to %s\n", addr)

	codec := knet.NewTLVPack()

	// Send three pings in a burst (TCP may deliver them as one segment).
	for i := 1; i <= 3; i++ {
		wire, err := codec.Pack(knet.NewMessage(1, []byte(fmt.Sprintf("ping-%d", i))))
		if err != nil {
			fatal("pack", err)
		}
		if _, err := conn.Write(wire); err != nil {
			fatal("write", err)
		}
	}
	fmt.Println("sent 3 pings")

	// Expect three pongs with matching payloads.
	for i := 1; i <= 3; i++ {
		msg, err := readMsg(conn, codec, 5*time.Second)
		if err != nil {
			fatal("read pong", err)
		}
		want := fmt.Sprintf("ping-%d", i)
		if msg.GetMsgID() != 2 || string(msg.GetData()) != want {
			fatalf("unexpected response: msgID=%d data=%q, want msgID=2 data=%q",
				msg.GetMsgID(), msg.GetData(), want)
		}
		fmt.Printf("pong %d/%d: %q\n", i, 3, string(msg.GetData()))
	}
	fmt.Println("echo OK")
}

// readMsg returns the next non-heartbeat message. It first drains messages
// already buffered inside the codec (TCP may deliver several in one segment),
// then reads from the socket only when the buffer is exhausted.
func readMsg(conn net.Conn, codec *knet.TLVPack, timeout time.Duration) (kiface.IMessage, error) {
	chunk := make([]byte, 4096)
	for {
		// Drain the codec's internal buffer first: Decode(nil) consumes no
		// socket data and returns any complete frames already received.
		msgs, derr := codec.Decode(nil)
		if derr != nil {
			return nil, derr
		}
		if m := firstNonHeartbeat(msgs); m != nil {
			return m, nil
		}
		// Buffer exhausted: read more from the socket.
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		n, err := conn.Read(chunk)
		if err != nil {
			return nil, err
		}
		msgs, derr = codec.Decode(chunk[:n])
		if derr != nil {
			return nil, derr
		}
		if m := firstNonHeartbeat(msgs); m != nil {
			return m, nil
		}
	}
}

func firstNonHeartbeat(msgs []kiface.IMessage) kiface.IMessage {
	for _, m := range msgs {
		if m.GetMsgID() != kiface.HeartBeatDefaultMsgID {
			return m
		}
	}
	return nil
}

func fatal(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
