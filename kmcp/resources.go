package kmcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerResources registers the Kinz management resources.
func (s *Server) registerResources() {
	add := func(uri, name, desc string, handler server.ResourceHandlerFunc) {
		s.mcpServer.AddResource(
			mcp.NewResource(uri, name,
				mcp.WithResourceDescription(desc),
				mcp.WithMIMEType("text/plain")),
			handler)
	}

	add("connections://", "Live connections", "Current live connections",
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return textContents("connections://", encodePretty(s.toolListConnections())), nil
		})

	add("metrics://", "Metrics", "Server metrics snapshot",
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return textContents("metrics://", encodePretty(s.toolGetMetrics())), nil
		})

	add("config://", "Configuration", "Effective server configuration",
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return textContents("config://", encodePretty(s.toolGetConfig())), nil
		})

	add("logs://", "Recent logs", "Recent log lines from the ring buffer",
		func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			if err := s.authorize("resources/read"); err != nil {
				return nil, err
			}
			out, err := s.toolGetLogs(100)
			if err != nil {
				return nil, err
			}
			return textContents("logs://", encodePretty(out)), nil
		})
}

func textContents(uri, text string) []mcp.ResourceContents {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "text/plain", Text: text},
	}
}
