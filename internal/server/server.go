package server

import (
	"log/slog"
	"net/http"

	"github.com/miamollie/solas/internal/chat"
	"github.com/miamollie/solas/internal/metrics"
)

type Server struct {
	handler http.Handler
	logger  *slog.Logger
	chat    *chat.Service
	metrics *metrics.Metrics
}

// New creates a server with baseline routes.
func New(logger *slog.Logger, client chat.Client, met *metrics.Metrics) *Server {
	if met == nil {
		met = metrics.New()
	}
	s := &Server{logger: logger, chat: chat.NewService(client), metrics: met}
	s.handler = s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.handler
}
