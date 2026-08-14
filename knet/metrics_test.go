package knet

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"kinz/kconf"
	"kinz/kiface"
)

// freePort returns a currently-free TCP port (small race, fine for tests).
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func TestServerMetricsEndpoint(t *testing.T) {
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

	port := freePort(t)
	if err := srv.AttachMetrics(fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
		t.Fatalf("AttachMetrics: %v", err)
	}

	// generate traffic: one echo round trip
	codec := NewTLVPack()
	conn := dialTLV(t, srv.Address())
	wire, _ := codec.Pack(NewMessage(1, []byte("x")))
	if _, err := conn.Write(wire); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, body := readMsg(t, conn, codec); string(body) != "x" {
		t.Fatalf("echo = %q, want x", body)
	}

	// fetch the metrics endpoint while the connection is still active
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/metrics", port))
	if err != nil {
		t.Fatalf("GET metrics: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	conn.Close()

	for _, want := range []string{
		"kinz_conns_total 1",
		"kinz_conns_active 1",
		"kinz_msgs_received_total 1",
		"kinz_msgs_sent_total 1",
		"kinz_msg_handle_duration_seconds_count 1",
		"# TYPE kinz_conns_active gauge",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, out)
		}
	}
}

func TestAttachMetricsTwice(t *testing.T) {
	srv := NewServer()
	port := freePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := srv.AttachMetrics(addr); err != nil {
		t.Fatalf("AttachMetrics: %v", err)
	}
	if err := srv.AttachMetrics(addr); err == nil {
		t.Fatal("expected error on second AttachMetrics")
	}
}
