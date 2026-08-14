package knet

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"kinz/kconf"
	"kinz/kiface"
)

// startTestServer runs a server on an ephemeral port and waits until ready.
func startTestServer(t *testing.T, cfg *kconf.Config) (kiface.IServer, context.CancelFunc) {
	t.Helper()
	srv := NewServer(WithConfig(cfg))
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Run(ctx) }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Address() != nil {
			return srv, cancel
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	t.Fatal("server did not become ready")
	return nil, nil
}

// testConfig builds a Config from defaults with loopback host and the given
// overrides (struct literals would zero out default fields).
func testConfig(overrides func(*kconf.Config)) *kconf.Config {
	cfg := kconf.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	if overrides != nil {
		overrides(cfg)
	}
	return cfg
}

// dialTLV connects a raw TLV-speaking client.
func dialTLV(t *testing.T, addr net.Addr) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

// readMsg reads one TLV message from conn (manual header parse; Decode is
// stream-based and cannot read a single message off a socket).
func readMsg(t *testing.T, conn net.Conn, codec *TLVPack) (uint32, []byte) {
	t.Helper()
	head := make([]byte, 8)
	if _, err := io.ReadFull(conn, head); err != nil {
		t.Fatalf("read head: %v", err)
	}
	dataLen := codec.order.Uint32(head[0:4])
	msgID := codec.order.Uint32(head[4:8])
	body := make([]byte, dataLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return msgID, body
}

func TestServerEcho(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 2
		c.MaxConn = 16
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	conn := dialTLV(t, srv.Address())
	defer conn.Close()
	codec := NewTLVPack()
	wire, err := codec.Pack(NewMessage(1, []byte("hi")))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if _, err := conn.Write(wire); err != nil {
		t.Fatalf("write: %v", err)
	}

	id, body := readMsg(t, conn, codec)
	if id != 2 || string(body) != "hi" {
		t.Fatalf("echo = (%d, %q), want (2, hi)", id, body)
	}
}

func TestServerStickyPackets(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 2
		c.MaxConn = 16
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	conn := dialTLV(t, srv.Address())
	defer conn.Close()
	codec := NewTLVPack()
	wire1, _ := codec.Pack(NewMessage(1, []byte("aa")))
	wire2, _ := codec.Pack(NewMessage(1, []byte("bb")))
	if _, err := conn.Write(append(wire1, wire2...)); err != nil { // sticky
		t.Fatalf("write: %v", err)
	}

	for _, want := range []string{"aa", "bb"} {
		id, body := readMsg(t, conn, codec)
		if id != 2 || string(body) != want {
			t.Fatalf("got (%d, %q), want (2, %q)", id, body, want)
		}
	}
}

func TestServerMaxConnRejection(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 1
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()

	conn1 := dialTLV(t, srv.Address())
	defer conn1.Close()
	// give the accept loop time to register conn1
	time.Sleep(50 * time.Millisecond)

	conn2 := dialTLV(t, srv.Address())
	defer conn2.Close()
	codec := NewTLVPack()
	_ = conn2.SetReadDeadline(time.Now().Add(2 * time.Second))

	id, body := readMsg(t, conn2, codec)
	if id != kiface.ServerFullMsgID {
		t.Fatalf("rejection msgID = %d, want %d", id, kiface.ServerFullMsgID)
	}
	if len(body) == 0 {
		t.Fatal("rejection body is empty")
	}
	// connection must then be closed by the server
	if _, err := conn2.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected EOF after rejection")
	}
}

func TestServerHeartbeatTimeout(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 16
		c.HeartbeatInterval = kconf.Duration(20 * time.Millisecond)
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()
	srv.StartHeartBeat(20 * time.Millisecond)

	conn := dialTLV(t, srv.Address())
	defer conn.Close()
	// client sends nothing: the server keeps sending heartbeats, then closes
	// the connection once the liveness timeout elapses. Read until EOF.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4096)
	for {
		if _, err := conn.Read(buf); err != nil {
			if errors.Is(err, io.EOF) {
				return // closed by the server's heartbeat timeout
			}
			t.Fatalf("expected EOF from heartbeat timeout, got %v", err)
		}
	}
}

func TestServerGracefulShutdown(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 2
		c.MaxConn = 16
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()

	conn := dialTLV(t, srv.Address())
	defer conn.Close()

	shutdownCtx, sc := context.WithTimeout(context.Background(), 2*time.Second)
	defer sc()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	// active connection must be closed
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected EOF after shutdown")
	}

	// idempotent
	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
}
