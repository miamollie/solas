package server

import (
	"log/slog"

	"github.com/miamollie/solas/internal/metrics"
	"github.com/miamollie/solas/internal/model"
)

type Server struct {
	logger    *slog.Logger
	llmClient model.Client
	metrics   *metrics.Metrics
}

// New creates a server with baseline routes.
func New(logger *slog.Logger, c model.Client, met *metrics.Metrics) *Server {
	if met == nil {
		met = metrics.New()
	}
	s := &Server{logger: logger, llmClient: c, metrics: met}
	return s
}
