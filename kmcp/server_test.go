package kmcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
	"kinz/knet"
)

var errAuth = errors.New("denied")

func newTestMCP(t *testing.T, opts ...Option) (*Server, kiface.IServer) {
	t.Helper()
	srv := knet.NewServer()
	mcp := NewServer(srv, opts...)
	return mcp, srv
}

// call feeds one JSON-RPC message and returns the decoded response.
func call(t *testing.T, mcp *Server, msg string) map[string]any {
	t.Helper()
	resp := mcp.handleMessage([]byte(msg))
	if resp == nil {
		t.Fatalf("no response for %s", msg)
	}
	var m map[string]any
	if err := json.Unmarshal(resp, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", resp, err)
	}
	return m
}

// toolText extracts the text content of a tools/call result.
func toolText(t *testing.T, resp map[string]any) string {
	t.Helper()
	if e, ok := resp["error"]; ok {
		t.Fatalf("unexpected error: %v", e)
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	return content["text"].(string)
}

func TestInitialize(t *testing.T) {
	mcp, _ := newTestMCP(t)
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	if resp["error"] != nil {
		t.Fatalf("initialize error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	if result["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocolVersion = %v", result["protocolVersion"])
	}
	si := result["serverInfo"].(map[string]any)
	if si["name"] != "kinz-mcp" {
		t.Fatalf("serverInfo = %v", si)
	}
}

func TestInitializeUnknownVersionFallsBack(t *testing.T) {
	mcp, _ := newTestMCP(t)
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2999-01-01"}}`)
	result := resp["result"].(map[string]any)
	if result["protocolVersion"] != latestProtocol {
		t.Fatalf("protocolVersion = %v, want latest %s", result["protocolVersion"], latestProtocol)
	}
}

func TestNotificationNoResponse(t *testing.T) {
	mcp, _ := newTestMCP(t)
	if resp := mcp.handleMessage([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); resp != nil {
		t.Fatalf("notification got response: %s", resp)
	}
}

func TestPing(t *testing.T) {
	mcp, _ := newTestMCP(t)
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if resp["error"] != nil {
		t.Fatalf("ping error: %v", resp["error"])
	}
}

func TestToolsList(t *testing.T) {
	mcp, _ := newTestMCP(t)
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	tools := resp["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 10 {
		t.Fatalf("tools = %d, want 10", len(tools))
	}
}

func TestServerInfoTool(t *testing.T) {
	mcp, srv := newTestMCP(t)
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"server_info","arguments":{}}}`)
	text := toolText(t, resp)
	if !strings.Contains(text, srv.Name()) {
		t.Fatalf("server_info missing name: %s", text)
	}
}

func TestUnknownMethod(t *testing.T) {
	mcp, _ := newTestMCP(t)
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":5,"method":"nope"}`)
	e := resp["error"].(map[string]any)
	if e["code"].(float64) != codeMethodNotFound {
		t.Fatalf("error = %v, want -32601", e)
	}
}

func TestParseError(t *testing.T) {
	mcp, _ := newTestMCP(t)
	resp := call(t, mcp, `{invalid`)
	e := resp["error"].(map[string]any)
	if e["code"].(float64) != codeParseError {
		t.Fatalf("error = %v, want -32700", e)
	}
}

func TestAuthDenied(t *testing.T) {
	mcp, _ := newTestMCP(t, WithAuth(func(method string) error { return errAuth }))
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"get_metrics","arguments":{}}}`)
	e := resp["error"].(map[string]any)
	if e["code"].(float64) != codeAuthDenied {
		t.Fatalf("error = %v, want auth denied", e)
	}
}

func TestUnknownTool(t *testing.T) {
	mcp, _ := newTestMCP(t)
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"no_such_tool","arguments":{}}}`)
	e := resp["error"].(map[string]any)
	if e["code"].(float64) != codeInvalidParams {
		t.Fatalf("error = %v, want -32602", e)
	}
}

func TestGetConfigTool(t *testing.T) {
	cfg := kconf.Default()
	mcp, _ := newTestMCP(t, WithConfig(cfg))
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"get_config","arguments":{}}}`)
	text := toolText(t, resp)
	if !strings.Contains(text, `"name": "KinzServer"`) {
		t.Fatalf("config missing name: %s", text)
	}
}

func TestGetLogsTool(t *testing.T) {
	ring := klog.NewRingBuffer(1024)
	_, _ = ring.Write([]byte("line1\nline2\n"))
	mcp, _ := newTestMCP(t, WithLogRing(ring))
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"get_logs","arguments":{"lines":10}}}`)
	text := toolText(t, resp)
	if !strings.Contains(text, "line1") || !strings.Contains(text, "line2") {
		t.Fatalf("logs missing lines: %s", text)
	}
}

func TestResourcesListAndRead(t *testing.T) {
	mcp, _ := newTestMCP(t)
	resp := call(t, mcp, `{"jsonrpc":"2.0","id":10,"method":"resources/list"}`)
	rs := resp["result"].(map[string]any)["resources"].([]any)
	if len(rs) != 4 {
		t.Fatalf("resources = %d, want 4", len(rs))
	}

	resp = call(t, mcp, `{"jsonrpc":"2.0","id":11,"method":"resources/read","params":{"uri":"metrics://"}}`)
	if resp["error"] != nil {
		t.Fatalf("read metrics: %v", resp["error"])
	}
	resp = call(t, mcp, `{"jsonrpc":"2.0","id":12,"method":"resources/read","params":{"uri":"nope://"}}`)
	if resp["error"] == nil {
		t.Fatal("expected error for unknown uri")
	}
}
