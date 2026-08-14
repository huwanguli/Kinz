package kmetrics

import (
	"bytes"
	"strings"
	"testing"
)

func TestCounter(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("kinz_test_total", "test counter")
	c.Inc()
	c.Inc()
	c.Add(3)
	if c.Value() != 5 {
		t.Fatalf("Value = %d, want 5", c.Value())
	}
	// idempotent creation returns the same metric
	if r.Counter("kinz_test_total", "test counter") != c {
		t.Fatal("Counter(name) returned a different instance")
	}
}

func TestHistogramObserve(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("kinz_test_duration", "test histogram", []float64{0.5, 1.0})

	// values are binary-exact so the sum assertion is exact
	for _, v := range []float64{0.25, 0.5, 0.75, 1.5} {
		h.Observe(v)
	}
	if h.Count() != 4 {
		t.Fatalf("Count = %d, want 4", h.Count())
	}

	snap := r.Snapshot()
	hs := snap.Histograms["kinz_test_duration"]
	if hs.Count != 4 || hs.Sum != 3.0 {
		t.Fatalf("snapshot count/sum = %d/%v, want 4/3.0", hs.Count, hs.Sum)
	}
	// cumulative le counts: <=0.5 -> 2, <=1.0 -> 3
	if len(hs.Counts) != 2 || hs.Counts[0] != 2 || hs.Counts[1] != 3 {
		t.Fatalf("cumulative counts = %v, want [2 3]", hs.Counts)
	}
}

func TestHistogramDefaultBuckets(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("kinz_test_default", "h", nil)
	if len(h.buckets) != len(DefaultBuckets) {
		t.Fatalf("buckets = %d, want %d defaults", len(h.buckets), len(DefaultBuckets))
	}
	h.Observe(0.001)
	if h.Count() != 1 {
		t.Fatal("observe failed")
	}
}

func TestGauge(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("kinz_test_active", "active connections")
	g.Inc()
	g.Inc()
	g.Dec()
	if g.Value() != 1 {
		t.Fatalf("Value = %d, want 1", g.Value())
	}
	g.Set(7)
	if g.Value() != 7 {
		t.Fatalf("Value = %d, want 7", g.Value())
	}
	snap := r.Snapshot()
	if snap.Gauges["kinz_test_active"] != 7 {
		t.Fatalf("gauge snapshot = %v", snap.Gauges)
	}
}

func TestSnapshotContainsAll(t *testing.T) {
	r := NewRegistry()
	r.Counter("kinz_a_total", "a").Inc()
	r.Histogram("kinz_b", "b", nil).Observe(1)

	snap := r.Snapshot()
	if snap.Counters["kinz_a_total"] != 1 {
		t.Fatalf("counter snapshot = %v", snap.Counters)
	}
	if _, ok := snap.Histograms["kinz_b"]; !ok {
		t.Fatal("histogram missing from snapshot")
	}
}

func TestWritePrometheusText(t *testing.T) {
	r := NewRegistry()
	r.Counter("kinz_conns_total", "total connections").Add(7)
	h := r.Histogram("kinz_duration", "handle duration", []float64{0.5, 1.0})
	h.Observe(0.25)
	h.Observe(0.75)

	var buf bytes.Buffer
	if err := r.WritePrometheusText(&buf); err != nil {
		t.Fatalf("WritePrometheusText: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"# HELP kinz_conns_total total connections",
		"# TYPE kinz_conns_total counter",
		"kinz_conns_total 7",
		"# TYPE kinz_duration histogram",
		`kinz_duration_bucket{le="0.5"} 1`,
		`kinz_duration_bucket{le="1"} 2`,
		`kinz_duration_bucket{le="+Inf"} 2`,
		"kinz_duration_sum 1",
		"kinz_duration_count 2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatFloatSpecialValues(t *testing.T) {
	if formatFloat(0.5) != "0.5" {
		t.Fatalf("formatFloat(0.5) = %s", formatFloat(0.5))
	}
	if formatFloat(1e6) != "1e+06" && formatFloat(1e6) != "1000000" {
		t.Fatalf("unexpected large float format: %s", formatFloat(1e6))
	}
}
