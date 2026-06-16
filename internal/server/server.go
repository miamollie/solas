package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/miamollie/greenops-local-llm/internal/httpx"
)

// Server provides HTTP handlers for greenopsd.
type Server struct {
	handler http.Handler
	logger  *slog.Logger
}

// New creates a server with baseline routes.
func New(logger *slog.Logger) *Server {
	mux := http.NewServeMux()
	s := &Server{logger: logger}

	mux.HandleFunc("GET /health", s.handleHealth)

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
