// Package prometheus bridges the zero-dependency kmetrics measurement layer
// to the official Prometheus client, as an opt-in alternative to the built-in
// hand-written text exporter. Importing this package pulls in the official
// client dependency tree; the core framework never does.
package prometheus

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"kinz/kmetrics"
)

// snapshotCollector mirrors the kmetrics registry into a prometheus.Collector
// by re-reading the snapshot on every Collect call (live values).
type snapshotCollector struct {
	reg *kmetrics.Registry
}

// Describe implements prometheus.Collector.
func (c *snapshotCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

// Collect implements prometheus.Collector.
func (c *snapshotCollector) Collect(ch chan<- prometheus.Metric) {
	snap := c.reg.Snapshot()
	for name, value := range snap.Counters {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(name, "", nil, nil),
			prometheus.CounterValue, float64(value))
	}
	for name, h := range snap.Histograms {
		buckets := make(map[float64]uint64, len(h.Buckets))
		for i, ub := range h.Buckets {
			buckets[ub] = h.Counts[i]
		}
		ch <- prometheus.MustNewConstHistogram(
			prometheus.NewDesc(name, "", nil, nil),
			h.Count, h.Sum, buckets)
	}
}

// NewHandler returns an http.Handler serving the registry's metrics in
// Prometheus format via the official client (promhttp).
func NewHandler(reg *kmetrics.Registry) (http.Handler, error) {
	if reg == nil {
		return nil, fmt.Errorf("kinz: nil metrics registry")
	}
	promReg := prometheus.NewRegistry()
	if err := promReg.Register(&snapshotCollector{reg: reg}); err != nil {
		return nil, err
	}
	return promhttp.HandlerFor(promReg, promhttp.HandlerOpts{}), nil
}
