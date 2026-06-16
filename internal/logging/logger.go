package logging

import (
	"log/slog"
	"os"
)

// NewJSONLogger creates a structured JSON logger.
func NewJSONLogger() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(handler)
}
