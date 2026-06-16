package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

const (
	defaultListenAddress = ":8000"
	defaultOllamaBaseURL = "http://localhost:11434"
)

// Config contains process-level settings.
type Config struct {
	ListenAddress string
	Ollama        OllamaConfig
}

// OllamaConfig stores upstream Ollama settings.
type OllamaConfig struct {
	BaseURL string
}

// LoadFromEnv builds config with sensible defaults and environment overrides.
func LoadFromEnv() Config {
	cfg := Config{
		ListenAddress: defaultListenAddress,
		Ollama: OllamaConfig{
			BaseURL: defaultOllamaBaseURL,
		},
	}

	if v := os.Getenv("GREENOPSD_LISTEN_ADDRESS"); v != "" {
		cfg.ListenAddress = v
	}
	if v := os.Getenv("GREENOPSD_OLLAMA_BASE_URL"); v != "" {
		cfg.Ollama.BaseURL = v
	}

	return cfg
}

// Validate verifies config values are syntactically valid.
func Validate(cfg Config) error {
	if _, err := url.ParseRequestURI(cfg.Ollama.BaseURL); err != nil {
		return fmt.Errorf("invalid ollama base url: %w", err)
	}
	if _, err := net.ResolveTCPAddr("tcp", cfg.ListenAddress); err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	return nil
}
