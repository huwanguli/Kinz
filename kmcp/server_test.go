package kmcp

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
	"kinz/knet"
)

var errAuth = errors.New("denied")

func newTestServer(t *testing.T, opts ...Option) (*Server, kiface.IServer) {
	t.Helper()
	srv := knet.NewServer()
	m := NewServer(srv, opts...)
	return m, srv
}

// newMCPClient connects an mcp-go client to the server's streamable-HTTP
// handler via httptest and completes the initialize handshake.
func newMCPClient(t *testing.T, ts *httptest.Server) *client.Client {
	t.Helper()
	c, err := client.NewStreamableHttpClient(ts.URL)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if _, err := c.Initialize(context.Background(), mcp.InitializeRequest{}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return c
}

// callTool invokes a tool and concatenates its text content.
func callTool(t *testing.T, c *client.Client, name string, args map[string]any) string {
	t.Helper()
	res, err := c.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: name, Arguments: args},
	})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	var sb strings.Builder
	for _, content := range res.Content {
		if tc, ok := content.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

func TestToolsListCount(t *testing.T) {
	m, _ := newTestServer(t)
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	tools, err := c.ListTools(context.Background(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) != 10 {
		t.Fatalf("tools = %d, want 10", len(tools.Tools))
	}
}

func TestServerInfoTool(t *testing.T) {
	m, srv := newTestServer(t)
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	text := callTool(t, c, "server_info", nil)
	if !strings.Contains(text, srv.Name()) {
		t.Fatalf("server_info missing name: %s", text)
	}
	if !strings.Contains(text, `"version": "0.1.0"`) {
		t.Fatalf("server_info missing version: %s", text)
	}
}

func TestGetConfigTool(t *testing.T) {
	cfg := kconf.Default()
	m, _ := newTestServer(t, WithConfig(cfg))
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	text := callTool(t, c, "get_config", nil)
	if !strings.Contains(text, `"name": "KinzServer"`) {
		t.Fatalf("config missing name: %s", text)
	}
}

func TestGetLogsTool(t *testing.T) {
	ring := klog.NewRingBuffer(1024)
	_, _ = ring.Write([]byte("line1\nline2\n"))
	m, _ := newTestServer(t, WithLogRing(ring))
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	text := callTool(t, c, "get_logs", map[string]any{"lines": 10})
	if !strings.Contains(text, "line1") || !strings.Contains(text, "line2") {
		t.Fatalf("logs missing lines: %s", text)
	}
}

func TestGetMetricsTool(t *testing.T) {
	m, _ := newTestServer(t)
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	text := callTool(t, c, "get_metrics", nil)
	if !strings.Contains(text, "kinz_conns_total") {
		t.Fatalf("metrics missing counters: %s", text)
	}
}

func TestAuthDenied(t *testing.T) {
	m, _ := newTestServer(t, WithAuth(func(method string) error { return errAuth }))
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	if _, err := c.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "get_metrics", Arguments: nil},
	}); err == nil || !strings.Contains(err.Error(), "authorization denied") {
		t.Fatalf("expected authorization denial, got %v", err)
	}
}

func TestUnknownTool(t *testing.T) {
	m, _ := newTestServer(t)
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	if _, err := c.CallTool(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "no_such_tool", Arguments: nil},
	}); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestResourcesListAndRead(t *testing.T) {
	m, _ := newTestServer(t)
	ts := httptest.NewServer(m.Handler())
	defer ts.Close()
	c := newMCPClient(t, ts)

	res, err := c.ListResources(context.Background(), mcp.ListResourcesRequest{})
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(res.Resources) != 4 {
		t.Fatalf("resources = %d, want 4", len(res.Resources))
	}

	read, err := c.ReadResource(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "metrics://"},
	})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(read.Contents) == 0 {
		t.Fatal("empty resource contents")
	}
}
