package server

import (
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
	httpx "github.com/miamollie/solas/internal/middleware"
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

	// pprof endpoints for profiling (enabled in debug/profile workflows).
	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	r.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	r.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	r.Handle("/debug/pprof/block", pprof.Handler("block"))

	r.Group(func(r chi.Router) {
		r.Use(httpx.LoggingMiddleware(s.logger))

		// Standard OpenAI-compatible endpoints for clients that expect them, proxying to Ollama.
		r.Route("/v1", func(r chi.Router) {
			r.Get("/models", s.handleModels)
			r.Post("/models/pull", s.handlePullModel)
			r.Post("/chat/completions", s.handleChat)
		})
	})

	return r
}
