package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"net/http"
	"time"

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
	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting greenopsd", "listen_address", cfg.ListenAddress)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-sigCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		logger.Error("server failed", "error", err)
		return
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
