package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"time"
)

const (
	defaultListenAddress = ":8000"
	defaultOllamaBaseURL = "http://localhost:11434"
	defaultRequestTimeout = 60 * time.Second
	defaultOllamaTimeout  = 60 * time.Second
	defaultStartupTimeout = 10 * time.Second
)

// Config contains process-level settings.
type Config struct {
	ListenAddress string
	Ollama        OllamaConfig
	RequestTimeout time.Duration
	OllamaTimeout  time.Duration
	StartupTimeout time.Duration
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
		RequestTimeout: defaultRequestTimeout,
		OllamaTimeout:  defaultOllamaTimeout,
		StartupTimeout: defaultStartupTimeout,
	}

	if v := os.Getenv("GREENOPSD_LISTEN_ADDRESS"); v != "" {
		cfg.ListenAddress = v
	}
	if v := os.Getenv("GREENOPSD_OLLAMA_BASE_URL"); v != "" {
		cfg.Ollama.BaseURL = v
	}
	if v := os.Getenv("GREENOPSD_REQUEST_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RequestTimeout = d
		}
	}
	if v := os.Getenv("GREENOPSD_OLLAMA_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.OllamaTimeout = d
		}
	}
	if v := os.Getenv("GREENOPSD_STARTUP_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.StartupTimeout = d
		}
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
	if cfg.RequestTimeout <= 0 || cfg.OllamaTimeout <= 0 || cfg.StartupTimeout <= 0 {
		return fmt.Errorf("timeouts must be greater than zero")
	}
	return nil
}
