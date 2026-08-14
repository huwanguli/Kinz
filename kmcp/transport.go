package kmcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
)

const maxLine = 4 * 1024 * 1024 // scanner limit for large log/metrics payloads

// ServeStdio serves MCP over stdin/stdout (newline-delimited JSON), the
// Claude Desktop convention.
func (s *Server) ServeStdio() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)
	enc := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		resp := s.handleMessage(scanner.Bytes())
		if resp != nil {
			if err := enc.Encode(json.RawMessage(resp)); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// ServeConn serves MCP over an existing net.Conn (newline-delimited JSON).
func (s *Server) ServeConn(conn net.Conn) error {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)
	enc := json.NewEncoder(conn)
	for scanner.Scan() {
		resp := s.handleMessage(scanner.Bytes())
		if resp != nil {
			if err := enc.Encode(json.RawMessage(resp)); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// ServeListener accepts MCP clients on ln and serves each in a goroutine.
func (s *Server) ServeListener(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			_ = s.ServeConn(c)
		}(conn)
	}
}

// ListenAndServe listens on addr and serves MCP clients (TCP, line-delimited
// JSON).
func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.ServeListener(ln)
}
