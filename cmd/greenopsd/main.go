package main

import (
	"net/http"

	"github.com/miamollie/greenops-local-llm/internal/config"
	"github.com/miamollie/greenops-local-llm/internal/logging"
	"github.com/miamollie/greenops-local-llm/internal/server"
)

func main() {
	cfg := config.LoadFromEnv()
	logger := logging.NewJSONLogger()
	srv := server.New(logger)

	logger.Info("starting greenopsd", "listen_address", cfg.ListenAddress)
	if err := http.ListenAndServe(cfg.ListenAddress, srv.Handler()); err != nil {
		logger.Error("server failed", "error", err)
	}
}
