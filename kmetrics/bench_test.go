package kmetrics

import (
	"fmt"
	"testing"
)

// BenchmarkCounterInc measures the hot-path metric increment (the per-message
// and per-byte counters in knet).
func BenchmarkCounterInc(b *testing.B) {
	reg := NewRegistry()
	c := reg.Counter("kinz_bench_msgs_total", "benchmark counter")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

// BenchmarkSnapshot measures gathering + converting the whole registry into
// the read model (used by the MCP get_metrics tool and promhttp scrape).
func BenchmarkSnapshot(b *testing.B) {
	reg := NewRegistry()
	for i := 0; i < 20; i++ {
		reg.Counter(fmt.Sprintf("kinz_bench_counter_%d_total", i), "bench")
	}
	reg.Histogram("kinz_bench_duration_seconds", "bench", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.Snapshot()
	}
}
