package kmcp

import (
	"context"
	"io"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

// Handler returns the MCP streamable-HTTP handler for mounting on your own
// HTTP mux (e.g. behind a router with other endpoints).
func (s *Server) Handler() http.Handler {
	return server.NewStreamableHTTPServer(s.mcpServer)
}

// ServeHTTP serves MCP over streamable HTTP on addr (blocking).
func (s *Server) ServeHTTP(addr string) error {
	return server.NewStreamableHTTPServer(s.mcpServer).Start(addr)
}

// ServeStdio serves MCP over stdin/stdout (the Claude Desktop convention).
func (s *Server) ServeStdio() error {
	return s.serveStdio(os.Stdin, os.Stdout)
}

// serveStdio serves MCP over the given reader/writer pair. It is the
// testable seam behind ServeStdio.
func (s *Server) serveStdio(in io.Reader, out io.Writer) error {
	ss := server.NewStdioServer(s.mcpServer)
	return ss.Listen(context.Background(), in, out)
}
