// Package kmetrics is the Kinz measurement layer, built on the standard Go
// metrics library prometheus/client_golang. It provides:
//
//   - named, deduplicated Counter / Gauge / Histogram handles (write-only,
//     client_golang semantics: reads go through Snapshot)
//   - a framework-neutral Snapshot for MCP and other consumers
//   - a ready-made promhttp.Handler for the /metrics endpoint
//
// Framework code only touches this package; client_golang is the
// battle-tested implementation underneath (labels, OpenMetrics, exemplars).
package kmetrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
)

// Counter is a monotonically increasing counter (write-only handle; read via
// Registry.Snapshot or the metrics endpoint).
type Counter struct {
	inner prometheus.Counter
}

// Inc increments the counter by one.
func (c *Counter) Inc() { c.inner.Inc() }

// Add increments the counter by v.
func (c *Counter) Add(v uint64) { c.inner.Add(float64(v)) }

// Gauge is a value that can go up and down (write-only handle).
type Gauge struct {
	inner prometheus.Gauge
}

// Inc increments the gauge by one.
func (g *Gauge) Inc() { g.inner.Inc() }

// Dec decrements the gauge by one.
func (g *Gauge) Dec() { g.inner.Dec() }

// Set sets the gauge to v.
func (g *Gauge) Set(v int64) { g.inner.Set(float64(v)) }

// Histogram observes values into fixed upper-bound buckets (write-only handle).
type Histogram struct {
	inner prometheus.Histogram
}

// DefaultBuckets are the Prometheus default duration buckets (seconds).
var DefaultBuckets = prometheus.DefBuckets

// Observe records one observation v.
func (h *Histogram) Observe(v float64) { h.inner.Observe(v) }

// Registry owns a named set of metrics over a prometheus.Registry.
// Metric creation is idempotent: requesting the same name twice returns the
// existing metric.
type Registry struct {
	mu         sync.RWMutex
	promReg    *prometheus.Registry
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		promReg:    prometheus.NewRegistry(),
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}
}

// Counter returns the counter registered under name, creating it on first use.
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
	c = &Counter{inner: prometheus.NewCounter(prometheus.CounterOpts{Name: name, Help: help})}
	r.promReg.MustRegister(c.inner)
	r.counters[name] = c
	return c
}

// Gauge returns the gauge registered under name, creating it on first use.
func (r *Registry) Gauge(name, help string) *Gauge {
	r.mu.RLock()
	g, ok := r.gauges[name]
	r.mu.RUnlock()
	if ok {
		return g
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g = &Gauge{inner: prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})}
	r.promReg.MustRegister(g.inner)
	r.gauges[name] = g
	return g
}

// Histogram returns the histogram registered under name, creating it on first
// use. Empty buckets fall back to prometheus.DefBuckets.
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
	h = &Histogram{inner: prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    name,
		Help:    help,
		Buckets: append([]float64(nil), buckets...),
	})}
	r.promReg.MustRegister(h.inner)
	r.histograms[name] = h
	return h
}

// Snapshot is a framework-neutral copy of all metrics (for MCP and exporters).
type Snapshot struct {
	Counters   map[string]uint64
	Gauges     map[string]int64
	Histograms map[string]HistogramSnapshot
}

// HistogramSnapshot is a point-in-time histogram (buckets are cumulative "le"
// counts, including the +Inf bucket).
type HistogramSnapshot struct {
	Buckets []float64
	Counts  []uint64
	Count   uint64
	Sum     float64
}

// Snapshot captures the current state of all metrics via the prometheus
// Gatherer.
func (r *Registry) Snapshot() Snapshot {
	families, err := r.promReg.Gather()
	if err != nil {
		return Snapshot{
			Counters:   map[string]uint64{},
			Gauges:     map[string]int64{},
			Histograms: map[string]HistogramSnapshot{},
		}
	}
	snap := Snapshot{
		Counters:   make(map[string]uint64, len(families)),
		Gauges:     make(map[string]int64, len(families)),
		Histograms: make(map[string]HistogramSnapshot, len(families)),
	}
	for _, family := range families {
		name := family.GetName()
		if len(family.Metric) == 0 {
			continue
		}
		switch family.GetType() {
		case dto.MetricType_COUNTER:
			snap.Counters[name] = uint64(family.Metric[0].GetCounter().GetValue())
		case dto.MetricType_GAUGE:
			snap.Gauges[name] = int64(family.Metric[0].GetGauge().GetValue())
		case dto.MetricType_HISTOGRAM:
			snap.Histograms[name] = histogramSnapshot(family.Metric[0].GetHistogram())
		}
	}
	return snap
}

func histogramSnapshot(h *dto.Histogram) HistogramSnapshot {
	hs := HistogramSnapshot{
		Count: h.GetSampleCount(),
		Sum:   h.GetSampleSum(),
	}
	for _, b := range h.GetBucket() {
		hs.Buckets = append(hs.Buckets, b.GetUpperBound())
		hs.Counts = append(hs.Counts, b.GetCumulativeCount())
	}
	return hs
}

// Handler returns a promhttp.Handler serving this registry's metrics.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.promReg, promhttp.HandlerOpts{})
}
