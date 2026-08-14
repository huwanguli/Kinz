package kmcp

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

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

// mcpClient is a minimal line-delimited JSON-RPC client.
type mcpClient struct {
	conn net.Conn
	rd   *bufio.Reader
}

func newMCPClient(t *testing.T, addr string) *mcpClient {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("mcp dial: %v", err)
	}
	return &mcpClient{conn: conn, rd: bufio.NewReader(conn)}
}

func (c *mcpClient) call(t *testing.T, msg string) map[string]any {
	t.Helper()
	if _, err := fmt.Fprintf(c.conn, "%s\n", msg); err != nil {
		t.Fatalf("mcp write: %v", err)
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := c.rd.ReadBytes('\n')
	if err != nil {
		t.Fatalf("mcp read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("mcp response unmarshal: %v (%s)", err, line)
	}
	return m
}

func (c *mcpClient) close() { _ = c.conn.Close() }

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

func TestIntegrationMCPOverTCP(t *testing.T) {
	srv, cancel := startEchoServer(t)
	defer cancel()

	// a raw TLV client holds one connection
	rawConn, err := net.Dial("tcp", srv.Address().String())
	if err != nil {
		t.Fatalf("raw dial: %v", err)
	}
	defer rawConn.Close()
	time.Sleep(50 * time.Millisecond) // let the server register it

	// start the MCP endpoint on an already-bound listener (no bind race)
	mcpLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer mcpLn.Close()
	mcp := NewServer(srv)
	go func() { _ = mcp.ServeListener(mcpLn) }()

	mc := newMCPClient(t, mcpLn.Addr().String())
	defer mc.close()

	// 1. initialize
	resp := mc.call(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	if resp["error"] != nil {
		t.Fatalf("initialize: %v", resp["error"])
	}

	// 2. tools/list mentions the key tools
	resp = mc.call(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := resp["result"].(map[string]any)["tools"].([]any)
	names := map[string]bool{}
	for _, tl := range tools {
		names[tl.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"list_connections", "send_to_connection", "broadcast", "get_metrics", "get_logs", "shutdown_server"} {
		if !names[want] {
			t.Fatalf("tools missing %s", want)
		}
	}

	// 3. list_connections -> 1 connection
	resp = mc.call(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_connections","arguments":{}}}`)
	text := toolText(t, resp)
	if !strings.Contains(text, `"count": 1`) {
		t.Fatalf("list_connections = %s", text)
	}

	// 4. send_to_connection -> raw client receives it
	resp = mc.call(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"send_to_connection","arguments":{"connID":1,"msgID":1,"data":"via-mcp"}}}`)
	toolText(t, resp)
	id, body := readTLV(t, rawConn)
	if id != 1 || string(body) != "via-mcp" {
		t.Fatalf("received (%d, %q), want (1, via-mcp)", id, body)
	}

	// 5. get_metrics -> counters present
	resp = mc.call(t, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get_metrics","arguments":{}}}`)
	text = toolText(t, resp)
	if !strings.Contains(text, "kinz_conns_total") {
		t.Fatalf("get_metrics missing counters: %s", text)
	}

	// 6. resources/read metrics://
	resp = mc.call(t, `{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"metrics://"}}`)
	if resp["error"] != nil {
		t.Fatalf("resources/read: %v", resp["error"])
	}

	// 7. close_connection -> raw client EOF
	resp = mc.call(t, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"close_connection","arguments":{"connID":1}}}`)
	toolText(t, resp)
	_ = rawConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := rawConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected EOF after close_connection")
	}

	// 8. shutdown_server -> ok
	resp = mc.call(t, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"shutdown_server","arguments":{}}}`)
	if resp["error"] != nil {
		t.Fatalf("shutdown_server: %v", resp["error"])
	}
}
