package kmetrics

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/prometheus/common/expfmt"
)

// TestWritePrometheusTextParsableByOfficialClient validates our hand-written
// text exposition output against the official Prometheus parser, so format
// drift fails the tests instead of breaking production scrapes.
func TestWritePrometheusTextParsableByOfficialClient(t *testing.T) {
	r := NewRegistry()
	r.Counter("kinz_conns_total", "total connections").Add(3)
	h := r.Histogram("kinz_msg_handle_duration", "handler duration", []float64{0.5, 1.0})
	h.Observe(0.25)
	h.Observe(0.75)

	var buf bytes.Buffer
	if err := r.WritePrometheusText(&buf); err != nil {
		t.Fatalf("WritePrometheusText: %v", err)
	}

	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("official parser rejected output: %v\n%s", err, buf.String())
	}

	// counter
	counter, ok := families["kinz_conns_total"]
	if !ok {
		t.Fatal("counter family missing")
	}
	if counter.GetMetric()[0].GetCounter().GetValue() != 3 {
		t.Fatalf("counter value = %v, want 3", counter.GetMetric()[0].GetCounter().GetValue())
	}

	// histogram: buckets + sum + count
	hist, ok := families["kinz_msg_handle_duration"]
	if !ok {
		t.Fatal("histogram family missing")
	}
	m := hist.GetMetric()[0].GetHistogram()
	if m.GetSampleCount() != 2 || m.GetSampleSum() != 1.0 {
		t.Fatalf("histogram count/sum = %d/%v, want 2/1.0", m.GetSampleCount(), m.GetSampleSum())
	}
	buckets := m.GetBucket()
	// The parser synthesizes the +Inf bucket, so 2 configured + 1 Inf = 3.
	if len(buckets) != 3 || buckets[0].GetCumulativeCount() != 1 || buckets[1].GetCumulativeCount() != 2 {
		t.Fatalf("histogram buckets = %v", buckets)
	}
	// +Inf bucket is included by the parser
	if ub := buckets[len(buckets)-1].GetUpperBound(); !math.IsInf(ub, 1) {
		t.Fatalf("missing +Inf bucket: %v", buckets)
	}
}
