package klog

import (
	"testing"
)

// FuzzRingBuffer feeds arbitrary byte sequences and capacities to the ring
// buffer. Write/Bytes/String/Lines must never panic and Lines must never
// return more entries than requested (when n > 0).
//
// Run with: go test ./klog/ -fuzz=FuzzRingBuffer -fuzztime=30s
func FuzzRingBuffer(f *testing.F) {
	f.Add([]byte("hello\nworld\n"), 64)
	f.Add([]byte{0x00, 0xFF, 0x0A, 0x0A}, 7)
	f.Add([]byte{}, 1024)

	f.Fuzz(func(t *testing.T, data []byte, capacity int) {
		if capacity <= 0 || capacity > 1<<20 {
			return
		}
		r := NewRingBuffer(capacity)
		if _, err := r.Write(data); err != nil {
			t.Fatalf("Write: %v", err)
		}
		r.Bytes()
		_ = r.String()
		lines := r.Lines(5)
		if len(lines) > 5 {
			t.Fatalf("Lines(5) returned %d entries", len(lines))
		}
	})
}
