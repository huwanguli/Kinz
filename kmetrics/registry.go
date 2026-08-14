// Package kmetrics is the Kinz measurement layer: type-safe atomic counters
// and histograms with zero external dependencies. Framework code only touches
// this package; exporters are pluggable:
//
//   - built-in Prometheus text format: Registry.WritePrometheusText
//   - official Prometheus client (opt-in): kmetrics/prometheus
//   - MCP (later phase): Registry.Snapshot
//
// Metric names follow the Prometheus convention: counters end with _total.
package kmetrics

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
)

// Counter is a monotonically increasing counter (Prometheus counter semantics).
type Counter struct {
	name  string
	help  string
	value atomic.Uint64
}

// Inc increments the counter by one.
func (c *Counter) Inc() { c.value.Add(1) }

// Add increments the counter by v.
func (c *Counter) Add(v uint64) { c.value.Add(v) }

// Value returns the current value.
func (c *Counter) Value() uint64 { return c.value.Load() }

// Histogram observes values into fixed upper-bound buckets (Prometheus
// histogram semantics: buckets are cumulative "le" counts).
type Histogram struct {
	name    string
	help    string
	buckets []float64 // sorted upper bounds; +Inf is implicit

	mu     sync.Mutex
	counts []uint64 // per-bucket counts (not cumulative)
	count  uint64
	sum    float64
}

// DefaultBuckets are the Prometheus default duration buckets (seconds).
var DefaultBuckets = []float64{
	0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
}

// Observe records one observation v.
func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.count++
	h.sum += v
	for i, ub := range h.buckets {
		if v <= ub {
			h.counts[i]++
			break
		}
	}
}

// Count returns the number of observations.
func (h *Histogram) Count() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

// Registry owns a named set of metrics. Metric creation is idempotent:
// requesting the same name twice returns the existing metric.
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	histograms map[string]*Histogram
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*Counter),
		histograms: make(map[string]*Histogram),
	}
}

// Counter returns the counter registered under name, creating it on first use.
// name should follow the Prometheus convention ([a-zA-Z_:][a-zA-Z0-9_:]*).
func (r *Registry) Counter(name, help string) *Counter {
	r.mu.RLock()
	c, ok := r.counters[name]
	r.mu.RUnlock()
	if ok {
		return c
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c = &Counter{name: name, help: help}
	r.counters[name] = c
	return c
}

// Histogram returns the histogram registered under name, creating it on first
// use. Empty buckets fall back to DefaultBuckets.
func (r *Registry) Histogram(name, help string, buckets []float64) *Histogram {
	r.mu.RLock()
	h, ok := r.histograms[name]
	r.mu.RUnlock()
	if ok {
		return h
	}
	if len(buckets) == 0 {
		buckets = DefaultBuckets
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[name]; ok {
		return h
	}
	h = &Histogram{name: name, help: help, buckets: append([]float64(nil), buckets...), counts: make([]uint64, len(buckets))}
	r.histograms[name] = h
	return h
}

// Snapshot is a framework-neutral copy of all metrics (for MCP and exporters).
type Snapshot struct {
	Counters   map[string]uint64
	Histograms map[string]HistogramSnapshot
}

// HistogramSnapshot is a point-in-time histogram.
type HistogramSnapshot struct {
	Buckets []float64 // upper bounds (excludes +Inf)
	Counts  []uint64  // cumulative counts per bucket (le semantics)
	Count   uint64
	Sum     float64
}

// Snapshot captures the current state of all metrics.
func (r *Registry) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := Snapshot{
		Counters:   make(map[string]uint64, len(r.counters)),
		Histograms: make(map[string]HistogramSnapshot, len(r.histograms)),
	}
	for name, c := range r.counters {
		snap.Counters[name] = c.Value()
	}
	for name, h := range r.histograms {
		h.mu.Lock()
		snap.Histograms[name] = histogramSnapshotLocked(h)
		h.mu.Unlock()
	}
	return snap
}

func histogramSnapshotLocked(h *Histogram) HistogramSnapshot {
	hs := HistogramSnapshot{
		Buckets: append([]float64(nil), h.buckets...),
		Counts:  make([]uint64, len(h.buckets)),
		Count:   h.count,
		Sum:     h.sum,
	}
	cum := uint64(0)
	for i, n := range h.counts {
		cum += n
		hs.Counts[i] = cum
	}
	return hs
}

// WritePrometheusText writes the metrics in the Prometheus text exposition
// format (https://prometheus.io/docs/instrumenting/exposition_formats/).
func (r *Registry) WritePrometheusText(w writer) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, c := range r.counters {
		if err := writeCounter(w, name, c.help, c.Value()); err != nil {
			return err
		}
	}
	for name, h := range r.histograms {
		h.mu.Lock()
		hs := histogramSnapshotLocked(h)
		h.mu.Unlock()
		if err := writeHistogram(w, name, h.help, hs); err != nil {
			return err
		}
	}
	return nil
}

type writer interface{ Write([]byte) (int, error) }

func writeCounter(w writer, name, help string, value uint64) error {
	if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value); err != nil {
		return err
	}
	return nil
}

func writeHistogram(w writer, name, help string, h HistogramSnapshot) error {
	if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n", name, help, name); err != nil {
		return err
	}
	for i, ub := range h.Buckets {
		if _, err := fmt.Fprintf(w, "%s_bucket{le=%q} %d\n", name, formatFloat(ub), h.Counts[i]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n%s_sum %s\n%s_count %d\n",
		name, h.Count, name, formatFloat(h.Sum), name, h.Count); err != nil {
		return err
	}
	return nil
}

// formatFloat renders a float in Prometheus syntax (special values included).
func formatFloat(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "+Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	default:
		return strconv.FormatFloat(v, 'g', -1, 64)
	}
}
