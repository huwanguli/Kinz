package kpool

import (
	"fmt"
	"testing"
)

// BenchmarkPoolGetPut measures the pooled Get+Put round trip per size class.
// With a warm pool this should be ~0 allocations: the hot path of the
// framework's per-connection read buffers and frame payloads.
func BenchmarkPoolGetPut(b *testing.B) {
	for _, size := range []int{4096, 16384, 65536} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				buf := Get(size)
				Put(buf)
			}
		})
	}
}

// BenchmarkPoolGetOnly measures Get on a cold pool (buffer allocation cost,
// i.e. the first-touch path before buffers circulate back).
func BenchmarkPoolGetOnly(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Get(4096)
	}
}

// BenchmarkPoolDirectAlloc measures the oversize path: requests beyond the
// largest size class (65536) allocate directly and are not pooled. This is the
// baseline to compare the pooled path against.
func BenchmarkPoolDirectAlloc(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Get(128 * 1024)
	}
}
