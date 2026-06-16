package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics stores the Prometheus registry and handler.
type Metrics struct {
	Registry *prometheus.Registry
	handler  http.Handler
	requests *prometheus.CounterVec
	clients  *prometheus.CounterVec
	duration *prometheus.HistogramVec
	prompt   *prometheus.CounterVec
	output   *prometheus.CounterVec
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
	clients := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "greenops_client_requests_total",
			Help: "Total number of requests by attributed client metadata.",
		},
		[]string{"model", "status", "client", "user_agent", "remote_ip"},
	)
	duration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "greenops_request_duration_seconds",
			Help:    "Duration of OpenAI-compatible requests handled by greenopsd.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"model"},
	)
	prompt := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "greenops_prompt_tokens_total",
			Help: "Total prompt tokens processed by greenopsd.",
		},
		[]string{"model"},
	)
	output := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "greenops_completion_tokens_total",
			Help: "Total completion tokens processed by greenopsd.",
		},
		[]string{"model"},
	)
	reg.MustRegister(requests, clients, duration, prompt, output)
	return &Metrics{
		Registry: reg,
		handler:  promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
		requests: requests,
		clients:  clients,
		duration: duration,
		prompt:   prompt,
		output:   output,
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

// IncClientRequest increments attributed request counters.
func (m *Metrics) IncClientRequest(model string, status int, client, userAgent, remoteIP string) {
	if model == "" {
		model = "unknown"
	}
	if client == "" {
		client = "unknown"
	}
	if userAgent == "" {
		userAgent = "unknown"
	}
	if remoteIP == "" {
		remoteIP = "unknown"
	}
	m.clients.WithLabelValues(model, strconv.Itoa(status), client, userAgent, remoteIP).Inc()
}

// ObserveDuration records request duration by model label.
func (m *Metrics) ObserveDuration(model string, d time.Duration) {
	if model == "" {
		model = "unknown"
	}
	m.duration.WithLabelValues(model).Observe(d.Seconds())
}

// AddTokenUsage increments token counters by model.
func (m *Metrics) AddTokenUsage(model string, promptTokens, completionTokens int) {
	if model == "" {
		model = "unknown"
	}
	m.prompt.WithLabelValues(model).Add(float64(promptTokens))
	m.output.WithLabelValues(model).Add(float64(completionTokens))
}
