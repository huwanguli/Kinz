package kmcp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

func TestServeStdio(t *testing.T) {
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdinR.Close()
	defer stdoutW.Close()

	m, _ := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = server.NewStdioServer(m.mcpServer).Listen(ctx, stdinR, stdoutW)
	}()

	// send an initialize request over the stdin pipe
	msg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n"
	if _, err := fmt.Fprint(stdinW, msg); err != nil {
		t.Fatal(err)
	}

	rd := bufio.NewReader(stdoutR)
	_ = stdoutR.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := rd.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(line), `"kinz-mcp"`) {
		t.Fatalf("unexpected response: %s", line)
	}

	// closing stdin ends the listen loop
	_ = stdinW.Close()
}
