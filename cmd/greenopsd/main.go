package main

import (
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

	ollamaClient, err := ollama.NewClient(cfg.Ollama.BaseURL, http.DefaultClient)
	if err != nil {
		logger.Error("invalid ollama configuration", "error", err)
		return
	}

	met := metrics.New()
	srv := server.New(logger, ollamaClient, met)

	logger.Info("starting greenopsd", "listen_address", cfg.ListenAddress)
	if err := http.ListenAndServe(cfg.ListenAddress, srv.Handler()); err != nil {
		logger.Error("server failed", "error", err)
	}
}
