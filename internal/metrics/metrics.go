package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics stores the Prometheus registry and handler.
type Metrics struct {
	Registry *prometheus.Registry
	handler  http.Handler
	requests *prometheus.CounterVec
}

// New creates a metrics registry and exporter handler.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	requests := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "greenops_requests_total",
			Help: "Total number of OpenAI-compatible requests handled by greenopsd.",
		},
		[]string{"model", "status"},
	)
	reg.MustRegister(requests)
	return &Metrics{
		Registry: reg,
		handler:  promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		requests: requests,
	}
}

// Handler returns the Prometheus scrape handler.
func (m *Metrics) Handler() http.Handler {
	return m.handler
}

// IncRequests increments request counters by model and status code.
func (m *Metrics) IncRequests(model string, status int) {
	if model == "" {
		model = "unknown"
	}
	m.requests.WithLabelValues(model, strconv.Itoa(status)).Inc()
}
