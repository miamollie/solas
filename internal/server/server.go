package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/miamollie/greenops-local-llm/internal/httpx"
)

// Server provides HTTP handlers for greenopsd.
type Server struct {
	handler http.Handler
	logger  *slog.Logger
	ready   readinessChecker
}

type readinessChecker interface {
	IsReachable(ctx context.Context) error
}

// New creates a server with baseline routes.
func New(logger *slog.Logger, ready readinessChecker) *Server {
	mux := http.NewServeMux()
	s := &Server{logger: logger, ready: ready}

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)

	h := httpx.RequestIDMiddleware(httpx.LoggingMiddleware(logger, mux))
	s.handler = h
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
	if s.ready == nil || s.ready.IsReachable(r.Context()) != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
