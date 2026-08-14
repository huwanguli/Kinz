package kmcp

import (
	"encoding/json"
)

// resourcesList returns the MCP resource directory.
func (s *Server) resourcesList() any {
	resources := []map[string]any{
		{"uri": "connections://", "name": "Live connections", "description": "Current live connections", "mimeType": "text/plain"},
		{"uri": "metrics://", "name": "Metrics", "description": "Server metrics snapshot", "mimeType": "text/plain"},
		{"uri": "config://", "name": "Configuration", "description": "Effective server configuration", "mimeType": "text/plain"},
		{"uri": "logs://", "name": "Recent logs", "description": "Recent log lines from the ring buffer", "mimeType": "text/plain"},
	}
	return map[string]any{"resources": resources}
}

func (s *Server) resourcesRead(params json.RawMessage) (any, *rpcError) {
	if rpcErr := s.authorize("resources/read"); rpcErr != nil {
		return nil, rpcErr
	}
	var p struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}

	var text string
	switch p.URI {
	case "connections://":
		text = encodePretty(s.toolListConnections())
	case "metrics://":
		text = encodePretty(s.toolGetMetrics())
	case "config://":
		text = encodePretty(s.toolGetConfig())
	case "logs://":
		var out any
		if err := s.toolGetLogs(nil, &out); err != nil {
			return nil, &rpcError{Code: codeInternalError, Message: err.Error()}
		}
		text = encodePretty(out)
	default:
		return nil, &rpcError{Code: codeInvalidParams, Message: "unknown resource uri: " + p.URI}
	}
	return map[string]any{
		"contents": []map[string]any{{"uri": p.URI, "mimeType": "text/plain", "text": text}},
	}, nil
}
