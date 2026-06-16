package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/miamollie/greenops-local-llm/internal/config"
	"github.com/miamollie/greenops-local-llm/internal/logging"
	"github.com/miamollie/greenops-local-llm/internal/metrics"
	"github.com/miamollie/greenops-local-llm/internal/ollama"
	"github.com/miamollie/greenops-local-llm/internal/server"
)

func main() {
	cfg := config.LoadFromEnv()
	logger := logging.NewJSONLogger()
	if err := config.Validate(cfg); err != nil {
		logger.Error("invalid configuration", "error", err)
		return
	}

	httpClient := &http.Client{Timeout: cfg.OllamaTimeout}
	ollamaClient, err := ollama.NewClient(cfg.Ollama.BaseURL, httpClient)
	if err != nil {
		logger.Error("invalid ollama configuration", "error", err)
		return
	}

	startupCtx, cancel := context.WithTimeout(context.Background(), cfg.StartupTimeout)
	defer cancel()
	if err := ollamaClient.IsReachable(startupCtx); err != nil {
		logger.Error("startup readiness check failed", "error", err)
		return
	}

	met := metrics.New()
	srv := server.New(logger, ollamaClient, met)
	handler := http.TimeoutHandler(srv.Handler(), cfg.RequestTimeout, `{"error":"request timeout"}`)

	logger.Info("starting greenopsd", "listen_address", cfg.ListenAddress)
	if err := http.ListenAndServe(cfg.ListenAddress, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "error", err)
	}
}
