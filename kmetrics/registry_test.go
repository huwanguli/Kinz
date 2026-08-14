package kmetrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCounter(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("kinz_test_total", "test counter")
	c.Inc()
	c.Inc()
	c.Add(3)

	snap := r.Snapshot()
	if snap.Counters["kinz_test_total"] != 5 {
		t.Fatalf("counter = %d, want 5", snap.Counters["kinz_test_total"])
	}
	if r.Counter("kinz_test_total", "test counter") != c {
		t.Fatal("Counter(name) returned a different instance")
	}
}

func TestGauge(t *testing.T) {
	r := NewRegistry()
	g := r.Gauge("kinz_test_active", "active connections")
	g.Inc()
	g.Inc()
	g.Dec()
	g.Set(7)

	snap := r.Snapshot()
	if snap.Gauges["kinz_test_active"] != 7 {
		t.Fatalf("gauge = %d, want 7", snap.Gauges["kinz_test_active"])
	}
}

func TestHistogramObserve(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("kinz_test_duration", "test histogram", []float64{0.5, 1.0})

	for _, v := range []float64{0.25, 0.5, 0.75, 1.5} {
		h.Observe(v)
	}

	snap := r.Snapshot()
	hs, ok := snap.Histograms["kinz_test_duration"]
	if !ok {
		t.Fatal("histogram missing from snapshot")
	}
	if hs.Count != 4 || hs.Sum != 3.0 {
		t.Fatalf("count/sum = %d/%v, want 4/3.0", hs.Count, hs.Sum)
	}
	// client_golang returns configured buckets only (the +Inf bucket is
	// implicit in the text format); cumulative counts: <=0.5 -> 2, <=1.0 -> 3.
	if len(hs.Buckets) != 2 || hs.Counts[0] != 2 || hs.Counts[1] != 3 {
		t.Fatalf("buckets/counts = %v / %v", hs.Buckets, hs.Counts)
	}
}

func TestHistogramDefaultBuckets(t *testing.T) {
	r := NewRegistry()
	h := r.Histogram("kinz_test_default", "h", nil)
	h.Observe(0.001)
	snap := r.Snapshot()
	if snap.Histograms["kinz_test_default"].Count != 1 {
		t.Fatal("observe failed")
	}
}

func TestSnapshotEmpty(t *testing.T) {
	r := NewRegistry()
	snap := r.Snapshot()
	if len(snap.Counters) != 0 || len(snap.Gauges) != 0 || len(snap.Histograms) != 0 {
		t.Fatalf("expected empty snapshot, got %+v", snap)
	}
}

func TestHandlerServesMetrics(t *testing.T) {
	r := NewRegistry()
	r.Counter("kinz_conns_total", "total connections").Add(7)
	r.Gauge("kinz_conns_active", "active").Set(2)
	r.Histogram("kinz_duration", "d", nil).Observe(0.25)

	ts := httptest.NewServer(r.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	out := string(body)
	for _, want := range []string{
		"kinz_conns_total 7",
		"kinz_conns_active 2",
		"kinz_duration_bucket",
		"kinz_duration_count 1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, out)
		}
	}
}
