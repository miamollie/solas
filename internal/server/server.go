package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/miamollie/solas/internal/llmclients"
	"github.com/miamollie/solas/internal/metrics"
)

// Server provides HTTP handlers for solas.
type Server struct {
	handler      http.Handler
	logger       *slog.Logger
	openAIClient llmclients.Client
	ollamaClient llmclients.Client
	metrics      *metrics.Metrics
}

// New creates a server with baseline routes.
func New(logger *slog.Logger, openAIClient llmclients.Client, ollamaClient llmclients.Client, met *metrics.Metrics) *Server {
	if met == nil {
		met = metrics.New()
	}
	s := &Server{logger: logger, openAIClient: openAIClient, ollamaClient: ollamaClient, metrics: met}
	s.handler = s.routes()
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.openAIClient == nil || s.ollamaClient == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	if s.openAIClient.Ready(r.Context()) != nil || s.ollamaClient.Ready(r.Context()) != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
