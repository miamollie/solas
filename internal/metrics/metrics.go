package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics stores the Prometheus registry and handler.
type Metrics struct {
	Registry *prometheus.Registry
	handler  http.Handler
}

// New creates a metrics registry and exporter handler.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return &Metrics{
		Registry: reg,
		handler:  promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
	}
}

// Handler returns the Prometheus scrape handler.
func (m *Metrics) Handler() http.Handler {
	return m.handler
}
