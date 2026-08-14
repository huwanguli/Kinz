package kmcp

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/knet"
)

// startEchoServer runs a knet server with an echo router on an ephemeral port.
func startEchoServer(t *testing.T) (kiface.IServer, context.CancelFunc) {
	t.Helper()
	cfg := kconf.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	srv := knet.NewServer(knet.WithConfig(cfg))
	if _, err := srv.AddRouterSlices(1, func(req kiface.IRequest) {
		_ = req.GetConnection().SendMsg(2, req.GetData())
	}); err != nil {
		t.Fatalf("AddRouterSlices: %v", err)
	}
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
	t.Fatal("server not ready")
	return nil, nil
}

// readTLV reads one TLV message from a raw connection (manual header parse).
func readTLV(t *testing.T, conn net.Conn) (uint32, []byte) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	head := make([]byte, 8)
	if _, err := io.ReadFull(conn, head); err != nil {
		t.Fatalf("read head: %v", err)
	}
	dataLen := binary.LittleEndian.Uint32(head[0:4])
	msgID := binary.LittleEndian.Uint32(head[4:8])
	body := make([]byte, dataLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return msgID, body
}

func TestIntegrationMCP(t *testing.T) {
	srv, cancel := startEchoServer(t)
	defer cancel()

	// a raw TLV client holds one connection
	rawConn, err := net.Dial("tcp", srv.Address().String())
	if err != nil {
		t.Fatalf("raw dial: %v", err)
	}
	defer rawConn.Close()
	time.Sleep(50 * time.Millisecond) // let the server register it

	m := NewServer(srv)
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	// list_connections -> 1 connection
	text := callTool(t, c, "list_connections", nil)
	if !strings.Contains(text, `"count": 1`) {
		t.Fatalf("list_connections = %s", text)
	}

	// send_to_connection -> raw client receives it
	callTool(t, c, "send_to_connection", map[string]any{"connID": 1, "msgID": 1, "data": "via-mcp"})
	id, body := readTLV(t, rawConn)
	if id != 1 || string(body) != "via-mcp" {
		t.Fatalf("received (%d, %q), want (1, via-mcp)", id, body)
	}

	// get_metrics -> counters present
	text = callTool(t, c, "get_metrics", nil)
	if !strings.Contains(text, "kinz_conns_total") {
		t.Fatalf("get_metrics missing counters: %s", text)
	}

	// resources/read metrics://
	read, err := c.ReadResource(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "metrics://"},
	})
	if err != nil {
		t.Fatalf("resources/read: %v", err)
	}
	if len(read.Contents) == 0 {
		t.Fatal("empty resource contents")
	}

	// close_connection -> raw client EOF
	callTool(t, c, "close_connection", map[string]any{"connID": 1})
	_ = rawConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := rawConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected EOF after close_connection")
	}

	// shutdown_server -> ok
	text = callTool(t, c, "shutdown_server", nil)
	if !strings.Contains(text, `"ok": true`) {
		t.Fatalf("shutdown_server = %s", text)
	}
}
