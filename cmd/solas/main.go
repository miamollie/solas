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
	"github.com/miamollie/solas/internal/logging"
	"github.com/miamollie/solas/internal/metrics"
	"github.com/miamollie/solas/internal/model"
	"github.com/miamollie/solas/internal/ollama"
	"github.com/miamollie/solas/internal/power"
	"github.com/miamollie/solas/internal/server"
)

func main() {
	if handled, exitCode := runCLI(os.Args[1:]); handled {
		os.Exit(exitCode)
	}

	runServer()
}

func runServer() {
	cfg := config.LoadFromEnv()
	logger := logging.NewJSONLogger()
	if err := config.Validate(cfg); err != nil {
		logger.Error("invalid configuration", "error", err)
		return
	}

	ollamaHTTPClient := &http.Client{Timeout: cfg.OllamaTimeout}
	ollamaClient, err := ollama.New(cfg.Ollama.BaseURL, ollamaHTTPClient)
	if err != nil {
		logger.Error("invalid ollama configuration", "error", err)
		return
	}

	met := metrics.New()
	srv := server.New(logger, model.Client(ollamaClient), met)
	handler := http.TimeoutHandler(srv.Handler(), cfg.RequestTimeout, `{"error":"request timeout"}`)
	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	profiler := power.NewProfilerWithMode(
		power.NewMacOSCollector(),
		met,
		logger,
		cfg.PowerInterval,
		cfg.Ollama.BaseURL,
		cfg.ProcessMode,
		cfg.Ollama.ContainerName,
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
