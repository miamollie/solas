package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/miamollie/solas/internal/middleware"
)

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.routes()
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)

	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)
	r.Handle("/metrics", s.metrics.Handler())

	r.Group(func(r chi.Router) {
		r.Use(httpx.LoggingMiddleware(s.logger))

		// Standard OpenAI-compatible endpoints for clients that expect them, proxying to Ollama.
		r.Route("/v1", func(r chi.Router) {
			r.Get("/models", s.handleModels)
			r.Post("/chat/completions", s.handleChat)
		})
	})

	return r
}
