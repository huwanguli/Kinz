package klog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultLogger(t *testing.T) {
	if L() == nil {
		t.Fatal("L() returned nil")
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, Level: LevelError})

	lg.Info("should be filtered")
	lg.Error("should be emitted")

	if strings.Contains(buf.String(), "should be filtered") {
		t.Fatal("Info was emitted despite LevelError")
	}
	if !strings.Contains(buf.String(), "should be emitted") {
		t.Fatalf("Error not emitted, got: %q", buf.String())
	}
}

func TestLevelVarDynamic(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, Level: LevelInfo})
	lg.SetLevel(LevelWarn)
	lg.Info("info line")
	if strings.Contains(buf.String(), "info line") {
		t.Fatal("Info emitted after raising level to Warn")
	}
}

func TestJSONHandler(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, JSON: true})

	lg.Info("hello", "key", "value")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v; got %q", err, buf.String())
	}
	if rec["msg"] != "hello" {
		t.Fatalf("msg = %v, want hello", rec["msg"])
	}
	if rec["key"] != "value" {
		t.Fatalf("key = %v, want value", rec["key"])
	}
}

func TestWithFields(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, JSON: true})

	lg.With("connID", 42).Info("conn event")

	var rec map[string]any
	_ = json.Unmarshal(buf.Bytes(), &rec)
	if rec["connID"] != float64(42) {
		t.Fatalf("connID = %v, want 42", rec["connID"])
	}
}

func TestInfofCompatibility(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, JSON: true})

	lg.InfoF("ping %d", 1)

	var rec map[string]any
	_ = json.Unmarshal(buf.Bytes(), &rec)
	if rec["msg"] != "ping 1" {
		t.Fatalf("msg = %v, want 'ping 1'", rec["msg"])
	}
}

func TestAddSource(t *testing.T) {
	var buf bytes.Buffer
	lg := New(Options{Output: &buf, AddSource: true})
	lg.Info("with source")
	// slog records the frame of the klog wrapper (log.go), proving AddSource works.
	if !strings.Contains(buf.String(), "source=") {
		t.Fatalf("output lacks source field: %q", buf.String())
	}
}

func TestSetDefault(t *testing.T) {
	old := L()
	defer SetDefault(old)

	var buf bytes.Buffer
	SetDefault(New(Options{Output: &buf, JSON: true}))
	if L() == old {
		t.Fatal("SetDefault did not replace the default logger")
	}
	Info("via package func")
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("package func output not JSON: %v", err)
	}
	if rec["msg"] != "via package func" {
		t.Fatalf("msg = %v, want 'via package func'", rec["msg"])
	}
}

func TestPackageLevelFuncs(t *testing.T) {
	old := L()
	defer SetDefault(old)

	var buf bytes.Buffer
	SetDefault(New(Options{Output: &buf, Level: LevelDebug}))

	Debug("d")
	Warn("w")
	ErrorF("e%d", 1)
	InfoF("i%d", 2)

	out := buf.String()
	for _, want := range []string{"msg=d", "msg=w", "msg=e1", "msg=i2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q: %q", want, out)
		}
	}
}
