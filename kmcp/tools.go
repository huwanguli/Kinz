package kmcp

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"kinz/kiface"
)

// registerTools registers the Kinz management tools on the MCP server.
func (s *Server) registerTools() {
	s.mcpServer.AddTool(
		mcp.NewTool("server_info", mcp.WithDescription("Returns the server name, version, listen address and uptime.")),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			return s.toolServerInfo()
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("list_connections", mcp.WithDescription("Lists all live connections (id, remote, local).")),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			return s.toolListConnections(), nil
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("get_connection", mcp.WithDescription("Returns details of one connection by id."),
			mcp.WithNumber("connID", mcp.Required(), mcp.Description("Connection id"))),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			id, ok := argUint(req, "connID")
			if !ok {
				return nil, fmt.Errorf("connID (integer) is required")
			}
			return s.toolGetConnection(id)
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("send_to_connection", mcp.WithDescription("Sends a message to one connection."),
			mcp.WithNumber("connID", mcp.Required(), mcp.Description("Connection id")),
			mcp.WithNumber("msgID", mcp.Required(), mcp.Description("Message id")),
			mcp.WithString("data", mcp.Required(), mcp.Description("Payload"))),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			connID, ok := argUint(req, "connID")
			if !ok {
				return nil, fmt.Errorf("connID (integer) is required")
			}
			msgID, ok := argUint(req, "msgID")
			if !ok {
				return nil, fmt.Errorf("msgID (integer) is required")
			}
			data, ok := argString(req, "data")
			if !ok {
				return nil, fmt.Errorf("data (string) is required")
			}
			return map[string]any{"ok": true}, s.toolSendToConnection(connID, uint32(msgID), data)
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("broadcast", mcp.WithDescription("Sends a message to every live connection."),
			mcp.WithNumber("msgID", mcp.Required(), mcp.Description("Message id")),
			mcp.WithString("data", mcp.Required(), mcp.Description("Payload"))),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			msgID, ok := argUint(req, "msgID")
			if !ok {
				return nil, fmt.Errorf("msgID (integer) is required")
			}
			data, ok := argString(req, "data")
			if !ok {
				return nil, fmt.Errorf("data (string) is required")
			}
			return s.toolBroadcast(uint32(msgID), data)
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("close_connection", mcp.WithDescription("Gracefully closes one connection."),
			mcp.WithNumber("connID", mcp.Required(), mcp.Description("Connection id"))),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			id, ok := argUint(req, "connID")
			if !ok {
				return nil, fmt.Errorf("connID (integer) is required")
			}
			return map[string]any{"ok": true}, s.toolCloseConnection(id)
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("get_metrics", mcp.WithDescription("Returns the server metrics (counters, gauges, histograms).")),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			return s.toolGetMetrics(), nil
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("get_config", mcp.WithDescription("Returns the effective server configuration.")),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			return s.toolGetConfig(), nil
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("get_logs", mcp.WithDescription("Returns the most recent log lines from the ring buffer."),
			mcp.WithNumber("lines", mcp.Description("Number of lines (default 50)"))),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			lines := 50
			if n, ok := argUint(req, "lines"); ok {
				lines = int(n)
			}
			return s.toolGetLogs(lines)
		}))

	s.mcpServer.AddTool(
		mcp.NewTool("shutdown_server", mcp.WithDescription("Gracefully shuts the server down.")),
		s.wrapTool(func(ctx context.Context, req mcp.CallToolRequest) (any, error) {
			return map[string]any{"ok": true}, s.toolShutdown()
		}))
}

func (s *Server) toolServerInfo() (any, error) {
	addr := ""
	if a := s.srv.Address(); a != nil {
		addr = a.String()
	}
	return map[string]any{
		"name":          s.srv.Name(),
		"version":       s.version,
		"address":       addr,
		"uptimeSeconds": int64(time.Since(s.startTime).Seconds()),
	}, nil
}

func (s *Server) toolListConnections() any {
	type connInfo struct {
		ID     uint64 `json:"id"`
		Remote string `json:"remote"`
		Local  string `json:"local"`
	}
	var conns []connInfo
	s.srv.GetConnMgr().Range(func(id uint64, c kiface.IConnection) bool {
		conns = append(conns, connInfo{ID: id, Remote: c.GetRemoteAddr().String(), Local: c.LocalAddr().String()})
		return true
	})
	return map[string]any{"count": len(conns), "connections": conns}
}

func (s *Server) toolGetConnection(id uint64) (any, error) {
	conn, err := s.srv.GetConnMgr().Get(id)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":     conn.GetConnID(),
		"remote": conn.GetRemoteAddr().String(),
		"local":  conn.LocalAddr().String(),
	}, nil
}

func (s *Server) toolSendToConnection(connID uint64, msgID uint32, data string) error {
	conn, err := s.srv.GetConnMgr().Get(connID)
	if err != nil {
		return err
	}
	return conn.SendMsg(msgID, []byte(data))
}

func (s *Server) toolBroadcast(msgID uint32, data string) (any, error) {
	count := 0
	s.srv.GetConnMgr().Range(func(_ uint64, c kiface.IConnection) bool {
		if err := c.SendMsg(msgID, []byte(data)); err == nil {
			count++
		}
		return true
	})
	return map[string]any{"sent": count}, nil
}

func (s *Server) toolCloseConnection(id uint64) error {
	conn, err := s.srv.GetConnMgr().Get(id)
	if err != nil {
		return err
	}
	conn.Stop()
	return nil
}

func (s *Server) toolGetMetrics() any {
	snap := s.srv.GetMetrics().Snapshot()
	return map[string]any{
		"counters":   snap.Counters,
		"gauges":     snap.Gauges,
		"histograms": snap.Histograms,
	}
}

func (s *Server) toolGetConfig() any {
	if s.cfg == nil {
		return map[string]any{}
	}
	return s.cfg
}

func (s *Server) toolGetLogs(lines int) (any, error) {
	if lines <= 0 {
		lines = 50
	}
	if s.ring == nil {
		return map[string]any{"logs": []string{}}, nil
	}
	return map[string]any{"logs": s.ring.Lines(lines)}, nil
}

func (s *Server) toolShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
