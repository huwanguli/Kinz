package klog

import (
	"io"
	"testing"
)

// BenchmarkRingBufferWrite measures appending a typical log line into the ring
// buffer (mutex + per-byte ring copy). It backs the MCP get_logs tool.
func BenchmarkRingBufferWrite(b *testing.B) {
	r := NewRingBuffer(64 * 1024)
	line := []byte("2026-08-14T12:00:00Z INFO conn 1 connected\n")
	b.SetBytes(int64(len(line)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.Write(line); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRingBufferLines measures reading the last N lines (copy + split).
func BenchmarkRingBufferLines(b *testing.B) {
	r := NewRingBuffer(64 * 1024)
	line := []byte("2026-08-14T12:00:00Z INFO conn 1 connected\n")
	for i := 0; i < 100; i++ {
		_, _ = r.Write(line)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := r.Lines(50); len(got) == 0 {
			b.Fatal("Lines returned empty")
		}
	}
}

// BenchmarkLogInfo measures one structured Info record through the slog
// handler (output discarded), the cost paid per framework log line.
func BenchmarkLogInfo(b *testing.B) {
	lg := New(Options{Output: io.Discard})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lg.Info("message received", "msgID", 1, "connID", 42)
	}
}

// BenchmarkLogInfoWithSource is BenchmarkLogInfo with AddSource enabled
// (file:line in every record).
func BenchmarkLogInfoWithSource(b *testing.B) {
	lg := New(Options{Output: io.Discard, AddSource: true})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lg.Info("message received", "msgID", 1, "connID", 42)
	}
}
