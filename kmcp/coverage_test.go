package kmcp

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// dialMCP connects an mcp-go client to a streamable-HTTP URL and completes
// the initialize handshake.
func dialMCP(t *testing.T, url string) *client.Client {
	t.Helper()
	c, err := client.NewStreamableHttpClient(url)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if _, err := c.Initialize(context.Background(), mcp.InitializeRequest{}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return c
}

func TestWithVersionOption(t *testing.T) {
	m, _ := newTestServer(t, WithVersion("9.9.9"))
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	text := callTool(t, c, "server_info", nil)
	if !strings.Contains(text, `"version": "9.9.9"`) {
		t.Fatalf("server_info version missing: %s", text)
	}
}

func TestGetConnectionAndBroadcastTools(t *testing.T) {
	srv, cancel := startEchoServer(t)
	defer cancel()

	connA, err := net.Dial("tcp", srv.Address().String())
	if err != nil {
		t.Fatalf("dial A: %v", err)
	}
	defer connA.Close()
	connB, err := net.Dial("tcp", srv.Address().String())
	if err != nil {
		t.Fatalf("dial B: %v", err)
	}
	defer connB.Close()

	// wait until the server registered both connections
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if srv.GetConnMgr().Len() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := srv.GetConnMgr().Len(); got != 2 {
		t.Fatalf("registered conns = %d, want 2", got)
	}

	m := NewServer(srv)
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	// get_connection with a live id
	text := callTool(t, c, "get_connection", map[string]any{"connID": 1})
	if !strings.Contains(text, `"id": 1`) || !strings.Contains(text, "127.0.0.1") {
		t.Fatalf("get_connection = %s", text)
	}

	// get_connection without connID -> tool error result (IsError content)
	if text := callTool(t, c, "get_connection", nil); !strings.Contains(text, "error:") {
		t.Fatalf("expected error content for missing connID, got: %s", text)
	}

	// get_connection with an unknown id -> tool error result
	if text := callTool(t, c, "get_connection", map[string]any{"connID": 999}); !strings.Contains(text, "error:") {
		t.Fatalf("expected error content for unknown connID, got: %s", text)
	}

	// broadcast reaches both connections
	text = callTool(t, c, "broadcast", map[string]any{"msgID": 99, "data": "hi-all"})
	if !strings.Contains(text, `"sent": 2`) {
		t.Fatalf("broadcast = %s, want sent 2", text)
	}
	for _, conn := range []net.Conn{connA, connB} {
		id, body := readTLV(t, conn)
		if id != 99 || string(body) != "hi-all" {
			t.Fatalf("broadcast received (%d, %q), want (99, hi-all)", id, body)
		}
	}
}

func TestServeHTTP(t *testing.T) {
	m, _ := newTestServer(t)

	// reserve a free port (small race window, acceptable in tests)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	go func() { _ = m.ServeHTTP(addr) }() // blocking; dies with the test process

	// connect and complete a handshake against the real listener. Start(addr)
	// mounts the handler at /mcp (mcp-go's default endpoint path), so the
	// client must target that path — unlike Handler(), which is the bare
	// handler and answers on any path the caller mounts it on.
	deadline := time.Now().Add(3 * time.Second)
	var c *client.Client
	for time.Now().Before(deadline) {
		probe, derr := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if derr == nil {
			_ = probe.Close()
			c = dialMCP(t, "http://"+addr+"/mcp")
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if c == nil {
		t.Fatal("ServeHTTP listener never became ready")
	}

	tools, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) != 10 {
		t.Fatalf("tools = %d, want 10", len(tools.Tools))
	}
}
