package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/miamollie/solas/internal/config"
	"github.com/miamollie/solas/internal/llmclients"
	"github.com/miamollie/solas/internal/logging"
	"github.com/miamollie/solas/internal/metrics"
	"github.com/miamollie/solas/internal/power"
	"github.com/miamollie/solas/internal/server"
)

func main() {
	cfg := config.LoadFromEnv()
	logger := logging.NewJSONLogger()
	if err := config.Validate(cfg); err != nil {
		logger.Error("invalid configuration", "error", err)
		return
	}

	ollamaHTTPClient := &http.Client{Timeout: cfg.OllamaTimeout}
	ollamaClient, err := llmclients.NewOllamaClient(cfg.Ollama.BaseURL, ollamaHTTPClient)
	if err != nil {
		logger.Error("invalid ollama configuration", "error", err)
		return
	}

	openAIHTTPClient := &http.Client{Timeout: cfg.OpenAITimeout}
	openAIClient, err := llmclients.NewOpenAIClient(cfg.OpenAI.BaseURL, cfg.OpenAI.APIKey, openAIHTTPClient)
	if err != nil {
		logger.Error("invalid openai configuration", "error", err)
		return
	}

	met := metrics.New()
	srv := server.New(logger, openAIClient, ollamaClient, met)
	handler := http.TimeoutHandler(srv.Handler(), cfg.RequestTimeout, `{"error":"request timeout"}`)
	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	profiler := power.NewProfiler(
		power.NewMacOSCollector(),
		met,
		logger,
		cfg.PowerInterval,
		map[string]string{
			"ollama": cfg.Ollama.BaseURL,
			"openai": cfg.OpenAI.BaseURL,
		},
	)
	go profiler.Start(sigCtx)

	go func() {
		logger.Info("starting solas", "listen_address", cfg.ListenAddress)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

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
