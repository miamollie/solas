package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/miamollie/solas/internal/chat"
	"github.com/miamollie/solas/internal/httpx"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)

	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady(chat.ProviderOllama))
	r.Handle("/metrics", s.metrics.Handler())

	r.Group(func(r chi.Router) {
		r.Use(httpx.LoggingMiddleware(s.logger))

		r.Route("/ollama/api", func(r chi.Router) {
			provider := chat.ProviderOllama
			r.Get("/tags", s.handleModels(provider))
			r.Post("/chat", s.handleChat(provider))
		})

		r.Route("/openai/v1", func(r chi.Router) {
			provider := chat.ProviderOpenAI
			r.Get("/models", s.handleModels(provider))
			r.Post("/chat/completions", s.handleChat(provider))
		})

		r.Route("/v1", func(r chi.Router) {
			provider := chat.ProviderOpenAI
			r.Get("/models", s.handleModels(provider))
			r.Post("/chat/completions", s.handleChat(provider))
		})
	})

	return r
}
