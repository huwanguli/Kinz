package klog

import (
	"strings"
	"sync"
)

// RingBuffer is a thread-safe, fixed-capacity byte ring buffer implementing
// io.Writer. When full, the oldest bytes are evicted. It is used as an
// optional log backend so the last N bytes/lines of output stay available
// (e.g. for the MCP get_logs tool in kmcp).
type RingBuffer struct {
	mu    sync.Mutex
	buf   []byte
	cap   int
	start int
	len   int
}

// NewRingBuffer creates a RingBuffer with the given byte capacity (must be > 0).
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 64 * 1024
	}
	return &RingBuffer{buf: make([]byte, capacity), cap: capacity}
}

// Write implements io.Writer: appends p, evicting the oldest bytes when full.
func (r *RingBuffer) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range p {
		if r.len < r.cap {
			r.buf[(r.start+r.len)%r.cap] = b
			r.len++
		} else {
			r.buf[r.start] = b
			r.start = (r.start + 1) % r.cap
		}
	}
	return len(p), nil
}

// Bytes returns a copy of the buffered content in chronological order.
func (r *RingBuffer) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, r.len)
	for i := 0; i < r.len; i++ {
		out[i] = r.buf[(r.start+i)%r.cap]
	}
	return out
}

// String returns the buffered content as a string.
func (r *RingBuffer) String() string {
	return string(r.Bytes())
}

// Lines returns the last n non-empty lines of the buffered content.
func (r *RingBuffer) Lines(n int) []string {
	content := r.String()
	if content == "" {
		return nil
	}
	parts := strings.Split(content, "\n")
	// Drop a trailing empty fragment produced by a final newline.
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) <= n {
		return parts
	}
	return parts[len(parts)-n:]
}
