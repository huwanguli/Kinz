// Package kmcp exposes a running Kinz server to AI tools via the Model
// Context Protocol (MCP), built on the mark3labs/mcp-go SDK. It is an
// OPT-IN adapter: the core framework never imports it, and it is linked into
// a binary only when the application explicitly imports it.
//
// Transports: stdio (Claude Desktop convention) and streamable HTTP (the
// standard modern MCP transport for remote clients).
//
// Usage:
//
//	mcp := kmcp.NewServer(srv, kmcp.WithConfig(cfg), kmcp.WithLogRing(ring))
//	go mcp.ServeHTTP("127.0.0.1:9001")   // streamable HTTP
//	mcp.ServeStdio()                       // stdio
package kmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
)

// Server is an MCP server bound to a Kinz server, wrapping an mcp-go
// MCPServer with the Kinz management tools and resources registered.
type Server struct {
	srv       kiface.IServer
	cfg       *kconf.Config
	ring      *klog.RingBuffer
	auth      AuthFunc
	version   string
	startTime time.Time

	mcpServer *server.MCPServer
}

// AuthFunc authorizes a tool/resource call; a non-nil error rejects it.
type AuthFunc func(method string) error

// Option customizes an MCP server.
type Option func(*Server)

// WithConfig exposes the server configuration via the config tool/resource.
func WithConfig(cfg *kconf.Config) Option {
	return func(s *Server) { s.cfg = cfg }
}

// WithLogRing exposes recent logs via the get_logs tool / logs resource.
func WithLogRing(ring *klog.RingBuffer) Option {
	return func(s *Server) { s.ring = ring }
}

// WithAuth installs an authorization callback invoked for tools/call and
// resources/read.
func WithAuth(f AuthFunc) Option {
	return func(s *Server) { s.auth = f }
}

// WithVersion overrides the server version reported to clients.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

// NewServer creates an MCP server bound to a Kinz server, registering the
// management tools and resources.
func NewServer(srv kiface.IServer, opts ...Option) *Server {
	s := &Server{
		srv:       srv,
		version:   "0.1.0",
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}
	s.mcpServer = server.NewMCPServer("kinz-mcp", s.version,
		server.WithInstructions("Manage a running Kinz server: inspect connections, send and broadcast messages, read metrics/config/logs, close connections, and shut down."),
		server.WithResourceCapabilities(true, false))
	s.registerTools()
	s.registerResources()
	return s
}

// authorize returns nil when the call is allowed.
func (s *Server) authorize(method string) error {
	if s.auth == nil {
		return nil
	}
	return s.auth(method)
}

// textResult builds a tool result with plain text content.
func textResult(text string, isError bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(text)},
		IsError: isError,
	}
}

func encodePretty(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}

// wrapTool adapts a plain tool implementation into an mcp-go handler with
// authorization and error-to-result conversion.
func (s *Server) wrapTool(f func(ctx context.Context, req mcp.CallToolRequest) (any, error)) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := s.authorize("tools/call"); err != nil {
			return nil, fmt.Errorf("authorization denied: %w", err)
		}
		result, err := f(ctx, req)
		if err != nil {
			return textResult(fmt.Sprintf("error: %v", err), true), nil
		}
		return textResult(encodePretty(result), false), nil
	}
}

// argMap extracts the tool arguments map (the wire type is `any`).
func argMap(req mcp.CallToolRequest) map[string]any {
	if m, ok := req.Params.Arguments.(map[string]any); ok {
		return m
	}
	return nil
}

// argString extracts a string argument.
func argString(req mcp.CallToolRequest, key string) (string, bool) {
	args := argMap(req)
	if args == nil {
		return "", false
	}
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// argUint extracts an integer argument (JSON numbers decode as float64).
func argUint(req mcp.CallToolRequest, key string) (uint64, bool) {
	args := argMap(req)
	if args == nil {
		return 0, false
	}
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return uint64(n), true
	case int:
		return uint64(n), true
	case int64:
		return uint64(n), true
	}
	return 0, false
}
