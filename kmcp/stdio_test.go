package kmcp

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
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
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = stdinR, stdoutW
	defer func() {
		os.Stdin, os.Stdout = oldIn, oldOut
	}()

	mcp, _ := newTestMCP(t)
	done := make(chan error, 1)
	go func() { done <- mcp.ServeStdio() }()

	// send a ping request
	if _, err := fmt.Fprintf(stdinW, `{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n"); err != nil {
		t.Fatal(err)
	}

	// read the response from the replaced stdout
	rd := bufio.NewReader(stdoutR)
	_ = stdoutR.SetReadDeadline(time.Now().Add(5 * time.Second))
	line, err := rd.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if !strings.Contains(string(line), `"result":{}`) {
		t.Fatalf("unexpected response: %s", line)
	}

	// closing stdin ends the serve loop
	_ = stdinW.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeStdio: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ServeStdio did not return after stdin closed")
	}
}
