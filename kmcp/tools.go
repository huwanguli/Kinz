package kmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kinz/kiface"
)

// tool describes an MCP tool.
type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func schema(required []string, props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func intProp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func (s *Server) toolsList() any {
	tools := []tool{
		{"server_info", "Returns the server name, version, listen address and uptime.", schema(nil, nil)},
		{"list_connections", "Lists all live connections (id, remote, local).", schema(nil, nil)},
		{"get_connection", "Returns details of one connection by id.", schema([]string{"connID"}, map[string]any{"connID": intProp("Connection id")})},
		{"send_to_connection", "Sends a message to one connection.", schema([]string{"connID", "msgID", "data"}, map[string]any{"connID": intProp("Connection id"), "msgID": intProp("Message id"), "data": strProp("Payload")})},
		{"broadcast", "Sends a message to every live connection.", schema([]string{"msgID", "data"}, map[string]any{"msgID": intProp("Message id"), "data": strProp("Payload")})},
		{"close_connection", "Gracefully closes one connection.", schema([]string{"connID"}, map[string]any{"connID": intProp("Connection id")})},
		{"get_metrics", "Returns the server metrics (counters, gauges, histograms).", schema(nil, nil)},
		{"get_config", "Returns the effective server configuration.", schema(nil, nil)},
		{"get_logs", "Returns the most recent log lines from the ring buffer.", schema(nil, map[string]any{"lines": intProp("Number of lines (default 50)")})},
		{"shutdown_server", "Gracefully shuts the server down.", schema(nil, nil)},
	}
	return map[string]any{"tools": tools}
}

func (s *Server) toolsCall(params json.RawMessage) (any, *rpcError) {
	if rpcErr := s.authorize("tools/call"); rpcErr != nil {
		return nil, rpcErr
	}
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}

	var result any
	var err error
	switch p.Name {
	case "server_info":
		result, err = s.toolServerInfo()
	case "list_connections":
		result = s.toolListConnections()
	case "get_connection":
		err = s.toolGetConnection(p.Arguments, &result)
	case "send_to_connection":
		err = s.toolSendToConnection(p.Arguments)
	case "broadcast":
		err = s.toolBroadcast(p.Arguments, &result)
	case "close_connection":
		err = s.toolCloseConnection(p.Arguments)
	case "get_metrics":
		result = s.toolGetMetrics()
	case "get_config":
		result = s.toolGetConfig()
	case "get_logs":
		err = s.toolGetLogs(p.Arguments, &result)
	case "shutdown_server":
		err = s.toolShutdown()
	default:
		return nil, &rpcError{Code: codeInvalidParams, Message: "unknown tool: " + p.Name}
	}
	if err != nil {
		return toolResult(fmt.Sprintf("error: %v", err), true), nil
	}
	return toolResult(encodePretty(result), false), nil
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}

func encodePretty(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
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

func (s *Server) toolGetConnection(args json.RawMessage, out *any) error {
	var a struct {
		ConnID uint64 `json:"connID"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return err
	}
	conn, err := s.srv.GetConnMgr().Get(a.ConnID)
	if err != nil {
		return err
	}
	*out = map[string]any{
		"id":     conn.GetConnID(),
		"remote": conn.GetRemoteAddr().String(),
		"local":  conn.LocalAddr().String(),
	}
	return nil
}

func (s *Server) toolSendToConnection(args json.RawMessage) error {
	var a struct {
		ConnID uint64 `json:"connID"`
		MsgID  uint32 `json:"msgID"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return err
	}
	conn, err := s.srv.GetConnMgr().Get(a.ConnID)
	if err != nil {
		return err
	}
	return conn.SendMsg(a.MsgID, []byte(a.Data))
}

func (s *Server) toolBroadcast(args json.RawMessage, out *any) error {
	var a struct {
		MsgID uint32 `json:"msgID"`
		Data  string `json:"data"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return err
	}
	count := 0
	s.srv.GetConnMgr().Range(func(_ uint64, c kiface.IConnection) bool {
		if err := c.SendMsg(a.MsgID, []byte(a.Data)); err == nil {
			count++
		}
		return true
	})
	*out = map[string]any{"sent": count}
	return nil
}

func (s *Server) toolCloseConnection(args json.RawMessage) error {
	var a struct {
		ConnID uint64 `json:"connID"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return err
	}
	conn, err := s.srv.GetConnMgr().Get(a.ConnID)
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

func (s *Server) toolGetLogs(args json.RawMessage, out *any) error {
	var a struct {
		Lines int `json:"lines"`
	}
	_ = json.Unmarshal(args, &a) // lines optional
	if a.Lines <= 0 {
		a.Lines = 50
	}
	if s.ring == nil {
		*out = map[string]any{"logs": []string{}}
		return nil
	}
	*out = map[string]any{"logs": s.ring.Lines(a.Lines)}
	return nil
}

func (s *Server) toolShutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
