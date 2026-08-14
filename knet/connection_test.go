package knet

import (
	"errors"
	"net"
	"testing"
	"time"

	"kinz/kconf"
	"kinz/kiface"
)

// tcpPair opens a real TCP pair for connection tests.
func tcpPair(t *testing.T) (client, server *net.TCPConn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accept := make(chan *net.TCPConn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			accept <- nil
			return
		}
		accept <- c.(*net.TCPConn)
	}()

	dialed, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	client = dialed.(*net.TCPConn)
	select {
	case server = <-accept:
	case <-time.After(2 * time.Second):
		t.Fatal("accept timeout")
	}
	return client, server
}

func newTestConnection(srv kiface.IServer, conn *net.TCPConn) *Connection {
	return NewConnection(srv, conn, 1, NewDataPack(), defaultDecoder(kconf.Default()),
		NewMsgHandler(0, 0), kconf.Default())
}

func TestConnectionProperty(t *testing.T) {
	srv := NewServer()
	client, sconn := tcpPair(t)
	defer client.Close()
	c := newTestConnection(srv, sconn)
	defer c.Stop()

	c.SetProperty("k", "v")
	v, err := c.GetProperty("k")
	if err != nil || v != "v" {
		t.Fatalf("GetProperty = %v, %v; want v, nil", v, err)
	}
	c.RemoveProperty("k")
	if _, err := c.GetProperty("k"); err == nil {
		t.Fatal("expected error after RemoveProperty")
	}
}

func TestConnectionSendMsgAfterStop(t *testing.T) {
	srv := NewServer()
	client, sconn := tcpPair(t)
	defer client.Close()
	c := newTestConnection(srv, sconn)

	c.Start()
	c.Stop()

	if err := c.SendMsg(1, []byte("x")); !errors.Is(err, kiface.ErrConnClosed) {
		t.Fatalf("SendMsg after Stop = %v, want ErrConnClosed", err)
	}
}

func TestConnectionStopIdempotent(t *testing.T) {
	srv := NewServer()
	client, sconn := tcpPair(t)
	defer client.Close()
	c := newTestConnection(srv, sconn)

	c.Start()
	c.Stop()
	c.Stop() // must not panic
}
