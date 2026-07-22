package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/miamollie/solas/internal/httpx"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)

	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)
	r.Handle("/metrics", s.metrics.Handler())

	r.Group(func(r chi.Router) {
		r.Use(httpx.LoggingMiddleware(s.logger))

		// Standard OpenAI-compatible endpoints for clients that expect them, proxying to Ollama upstream.
		r.Route("/v1", func(r chi.Router) {
			r.Get("/models", s.handleModels)
			r.Post("/chat/completions", s.handleChat)
		})
	})

	return r
}
