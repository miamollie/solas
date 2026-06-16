package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/miamollie/greenops-local-llm/internal/httpx"
	"github.com/miamollie/greenops-local-llm/internal/ollama"
	"github.com/miamollie/greenops-local-llm/internal/openai"
)

// Server provides HTTP handlers for greenopsd.
type Server struct {
	handler http.Handler
	logger  *slog.Logger
	ready   readinessChecker
}

type readinessChecker interface {
	IsReachable(ctx context.Context) error
	ListModels(ctx context.Context) ([]ollama.TagModel, error)
}

// New creates a server with baseline routes.
func New(logger *slog.Logger, ready readinessChecker) *Server {
	mux := http.NewServeMux()
	s := &Server{logger: logger, ready: ready}

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("GET /v1/models", s.handleModels)

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

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if s.ready == nil {
		http.Error(w, "ollama client unavailable", http.StatusBadGateway)
		return
	}
	models, err := s.ready.ListModels(r.Context())
	if err != nil {
		s.logger.Error("list models failed", "error", err)
		status := http.StatusBadGateway
		if errors.Is(err, context.Canceled) {
			status = http.StatusRequestTimeout
		}
		http.Error(w, "upstream error", status)
		return
	}

	resp := openai.ModelsResponse{Object: "list", Data: make([]openai.ModelInfo, 0, len(models))}
	for _, m := range models {
		resp.Data = append(resp.Data, openai.ModelInfo{ID: m.Name, Object: "model", OwnedBy: "ollama"})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
