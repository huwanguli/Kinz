package knet

import (
	"net"
	"strconv"
	"testing"
	"time"

	"kinz/kconf"
	"kinz/kiface"
)

func TestReconnectBackoffValue(t *testing.T) {
	p := reconnectPolicy{initial: 100 * time.Millisecond, max: 800 * time.Millisecond, multiplier: 2}
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 800 * time.Millisecond}, // capped
		{10, 800 * time.Millisecond},
	}
	for _, tc := range cases {
		if got := p.backoffValue(tc.attempt); got != tc.want {
			t.Fatalf("backoffValue(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestClientStartDialError(t *testing.T) {
	// nothing listens on this port
	c := NewClient("127.0.0.1", 1)
	if err := c.Start(); err == nil {
		t.Fatal("expected dial error")
	}
	// Start again must work after a failed start
	if err := c.Start(); err == nil {
		t.Fatal("expected dial error on second start")
	}
}

// splitAddr extracts host and port from a net.Addr.
func splitAddr(t *testing.T, addr net.Addr) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		t.Fatalf("split addr %v: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}
	return host, port
}

func TestClientEcho(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 16
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	host, port := splitAddr(t, srv.Address())
	client := NewClient(host, port, WithReconnect(50*time.Millisecond, 200*time.Millisecond, 2))
	responses := make(chan string, 3)
	if _, err := client.AddRouterSlices(2, func(req kiface.IRequest) {
		responses <- string(req.GetData())
	}); err != nil {
		t.Fatalf("client AddRouterSlices: %v", err)
	}
	if err := client.Start(); err != nil {
		t.Fatalf("client Start: %v", err)
	}
	defer client.Stop()

	for i := 0; i < 3; i++ {
		want := string(rune('a' + i))
		if err := client.Conn().SendMsg(1, []byte(want)); err != nil {
			t.Fatalf("SendMsg: %v", err)
		}
		select {
		case got := <-responses:
			if got != want {
				t.Fatalf("echo = %q, want %q", got, want)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for echo")
		}
	}
}

func TestClientReconnect(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 16
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}

	host, port := splitAddr(t, srv.Address())
	connStarts := make(chan struct{}, 8)
	client := NewClient(host, port, WithReconnect(30*time.Millisecond, 150*time.Millisecond, 2))
	client.SetOnConnStart(func(kiface.IConnection) { connStarts <- struct{}{} })
	if err := client.Start(); err != nil {
		t.Fatalf("client Start: %v", err)
	}
	defer client.Stop()

	// wait for the first connect
	<-connStarts

	// server force-closes the client's connection (client connID is 1)
	conn, err := srv.GetConnMgr().Get(1)
	if err != nil {
		t.Fatalf("get server-side conn: %v", err)
	}
	conn.Stop()

	// client must reconnect automatically
	select {
	case <-connStarts:
	case <-time.After(3 * time.Second):
		t.Fatal("client did not reconnect")
	}

	// and echo works again on the new connection
	responses := make(chan string, 1)
	if _, err := client.AddRouterSlices(2, func(req kiface.IRequest) {
		responses <- string(req.GetData())
	}); err != nil {
		t.Fatalf("client AddRouterSlices: %v", err)
	}
	if err := client.Conn().SendMsg(1, []byte("again")); err != nil {
		t.Fatalf("SendMsg: %v", err)
	}
	select {
	case got := <-responses:
		if got != "again" {
			t.Fatalf("echo = %q, want again", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for echo after reconnect")
	}
}

func TestClientStopNoReconnect(t *testing.T) {
	cfg := testConfig(func(c *kconf.Config) {
		c.WorkerPoolSize = 1
		c.MaxConn = 16
	})
	srv, cancel := startTestServer(t, cfg)
	defer cancel()

	host, port := splitAddr(t, srv.Address())
	connStarts := make(chan struct{}, 8)
	client := NewClient(host, port, WithReconnect(20*time.Millisecond, 100*time.Millisecond, 2))
	client.SetOnConnStart(func(kiface.IConnection) { connStarts <- struct{}{} })
	if err := client.Start(); err != nil {
		t.Fatalf("client Start: %v", err)
	}
	<-connStarts

	client.Stop()
	// Give a would-be reconnect attempt time to fire.
	time.Sleep(300 * time.Millisecond)
	select {
	case <-connStarts:
		t.Fatal("client reconnected after Stop")
	default:
	}
}
