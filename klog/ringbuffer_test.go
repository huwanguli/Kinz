package klog

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestRingBufferBasic(t *testing.T) {
	r := NewRingBuffer(16)
	if _, err := r.Write([]byte("hello ")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Write([]byte("world")); err != nil {
		t.Fatal(err)
	}
	if got := r.String(); got != "hello world" {
		t.Fatalf("String = %q, want %q", got, "hello world")
	}
}

func TestRingBufferEvictsOldest(t *testing.T) {
	r := NewRingBuffer(8)
	if _, err := r.Write([]byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Write([]byte("ij")); err != nil { // exceeds capacity by 2
		t.Fatal(err)
	}
	if got := r.String(); got != "cdefghij" {
		t.Fatalf("String = %q, want %q (oldest evicted)", got, "cdefghij")
	}
}

func TestRingBufferWriteLargerThanCapacity(t *testing.T) {
	r := NewRingBuffer(4)
	if _, err := r.Write([]byte("abcdef")); err != nil {
		t.Fatal(err)
	}
	// Only the last 4 bytes are kept.
	if got := r.String(); got != "cdef" {
		t.Fatalf("String = %q, want %q", got, "cdef")
	}
}

func TestRingBufferLines(t *testing.T) {
	r := NewRingBuffer(1024)
	fmt.Fprintf(r, "line1\nline2\nline3\n")
	lines := r.Lines(2)
	if len(lines) != 2 || lines[0] != "line2" || lines[1] != "line3" {
		t.Fatalf("Lines(2) = %v, want [line2 line3]", lines)
	}
	if all := r.Lines(10); len(all) != 3 {
		t.Fatalf("Lines(10) = %v, want 3 lines", all)
	}
	if empty := NewRingBuffer(8).Lines(2); empty != nil {
		t.Fatalf("Lines on empty = %v, want nil", empty)
	}
}

func TestRingBufferEmptyFragment(t *testing.T) {
	r := NewRingBuffer(64)
	fmt.Fprintf(r, "a\n\nb\n")
	lines := r.Lines(5)
	// empty middle line is kept, trailing empty fragment is dropped
	if len(lines) != 3 || lines[0] != "a" || lines[1] != "" || lines[2] != "b" {
		t.Fatalf("Lines = %v, want [a  b]", lines)
	}
}

func TestRingBufferConcurrentWrites(t *testing.T) {
	r := NewRingBuffer(64 * 1024)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_, _ = r.Write([]byte(fmt.Sprintf("g%d-%d\n", n, j)))
			}
		}(i)
	}
	wg.Wait()
	// Total written = 8*200 lines; buffer capacity 64KB keeps everything
	// (each line is short), so all lines must be present in order.
	content := r.String()
	for i := 0; i < 8; i++ {
		if !strings.Contains(content, fmt.Sprintf("g%d-199\n", i)) {
			t.Fatalf("missing line from goroutine %d", i)
		}
	}
}
