// Package kmcp exposes a running Kinz server to AI tools via the Model
// Context Protocol (MCP): JSON-RPC 2.0 over newline-delimited JSON. It is a
// zero-dependency hand-rolled subset: initialize handshake, tools/list,
// tools/call, resources/list, resources/read, ping, notifications.
//
// Transports: stdio (Claude Desktop style) and TCP (same line-delimited JSON).
//
// Usage:
//
//	mcp := kmcp.NewServer(srv, kmcp.WithConfig(cfg), kmcp.WithLogRing(ring))
//	mcp.ListenAndServe("127.0.0.1:9001")   // TCP
//	mcp.ServeStdio()                        // stdio
package kmcp

import (
	"encoding/json"
	"fmt"
	"time"

	"kinz/kconf"
	"kinz/kiface"
	"kinz/klog"
)

// Supported MCP protocol versions (responds with the client's version when
// known, otherwise with the latest).
var supportedProtocols = map[string]bool{
	"2024-11-05": true,
	"2025-06-18": true,
}

const latestProtocol = "2025-06-18"

// Server is an MCP server bound to a Kinz server.
type Server struct {
	srv     kiface.IServer
	cfg     *kconf.Config
	ring    *klog.RingBuffer
	auth    AuthFunc
	version string

	startTime time.Time
}

// AuthFunc authorizes a tool/resource method; a non-nil error rejects the call.
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

// WithAuth installs an authorization callback for tools/call and
// resources/read (called with the method name, e.g. "tools/call").
func WithAuth(f AuthFunc) Option {
	return func(s *Server) { s.auth = f }
}

// WithVersion overrides the server version reported to clients.
func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

// NewServer creates an MCP server bound to a Kinz server.
func NewServer(srv kiface.IServer, opts ...Option) *Server {
	s := &Server{
		srv:       srv,
		version:   "0.1.0",
		startTime: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// rpcRequest is a JSON-RPC 2.0 request or notification.
type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"` // nil => notification
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// rpcError is a JSON-RPC 2.0 error object.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcError        `json:"error,omitempty"`
}

// JSON-RPC error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
	codeAuthDenied     = -32001
)

// handleMessage processes one JSON-RPC message and returns the response bytes,
// or nil for notifications.
func (s *Server) handleMessage(raw []byte) []byte {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return s.buildResponse(nil, nil, &rpcError{Code: codeParseError, Message: "parse error: " + err.Error()})
	}
	if req.Method == "" {
		return s.buildResponse(nil, nil, &rpcError{Code: codeInvalidRequest, Message: "invalid request"})
	}
	if req.ID == nil {
		s.notify(req)
		return nil
	}
	result, rpcErr := s.dispatch(req)
	return s.buildResponse(req.ID, result, rpcErr)
}

func (s *Server) buildResponse(id *json.RawMessage, result any, rpcErr *rpcError) []byte {
	resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: result, Error: rpcErr}
	data, err := json.Marshal(resp)
	if err != nil {
		resp = rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: codeInternalError, Message: err.Error()}}
		data, _ = json.Marshal(resp)
	}
	return data
}

func (s *Server) notify(req rpcRequest) {
	// notifications carry no response; initialized is the only one we ack.
}

func (s *Server) dispatch(req rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req.Params)
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return s.toolsList(), nil
	case "tools/call":
		return s.toolsCall(req.Params)
	case "resources/list":
		return s.resourcesList(), nil
	case "resources/read":
		return s.resourcesRead(req.Params)
	default:
		return nil, &rpcError{Code: codeMethodNotFound, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}
}

func (s *Server) handleInitialize(params json.RawMessage) (any, *rpcError) {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	// params may be absent; ignore decode errors.
	_ = json.Unmarshal(params, &p)
	proto := p.ProtocolVersion
	if !supportedProtocols[proto] {
		proto = latestProtocol
	}
	return map[string]any{
		"protocolVersion": proto,
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "kinz-mcp",
			"version": s.version,
		},
	}, nil
}

// authorize rejects a call when the auth callback denies it.
func (s *Server) authorize(method string) *rpcError {
	if s.auth == nil {
		return nil
	}
	if err := s.auth(method); err != nil {
		return &rpcError{Code: codeAuthDenied, Message: "authorization denied: " + err.Error()}
	}
	return nil
}
