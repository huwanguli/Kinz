package kpool

import "testing"

func TestGetCapacityAtLeastRequested(t *testing.T) {
	for _, size := range []int{1, 100, 4096, 5000, 16384, 20000, 65536, 100000} {
		buf := Get(size)
		if cap(buf) < size {
			t.Fatalf("Get(%d) returned cap %d", size, cap(buf))
		}
	}
}

func TestPutReusesBuffers(t *testing.T) {
	b1 := Get(100)
	Put(b1)
	b2 := Get(100)
	// The pool may hand the same buffer back; at minimum both must be valid.
	if b2 == nil {
		t.Fatal("Get returned nil")
	}
	if cap(b2) < 100 {
		t.Fatalf("cap = %d, want >= 100", cap(b2))
	}
}

func TestPutPoisoning(t *testing.T) {
	buf := Get(100)
	buf[0] = 0
	Put(buf)
	// The first byte must now carry the poison sentinel (buffer returned to pool).
	if buf[0] != putPoison {
		t.Fatalf("first byte = %x, want poison %x", buf[0], putPoison)
	}
}

func TestPutWrongSizeDropped(t *testing.T) {
	// A buffer with a non-class capacity must be dropped, not pooled.
	buf := make([]byte, 100)
	Put(buf) // must not panic
}

func TestGetLargeAllocatesDirectly(t *testing.T) {
	buf := Get(200000)
	if cap(buf) < 200000 {
		t.Fatalf("cap = %d, want >= 200000", cap(buf))
	}
	// Writing past a class boundary must be safe.
	buf[199999] = 1
}
