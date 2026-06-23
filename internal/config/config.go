package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"time"
)

const (
	defaultListenAddress  = ":8000"
	defaultOllamaBaseURL  = "http://127.0.0.1:11434"
	defaultOpenAIBaseURL  = "https://api.openai.com"
	defaultRequestTimeout = 60 * time.Second
	defaultOllamaTimeout  = 60 * time.Second
	defaultOpenAITimeout  = 60 * time.Second
	defaultStartupTimeout = 10 * time.Second
	defaultPowerInterval  = 5 * time.Second
)

// Config contains process-level settings.
type Config struct {
	ListenAddress  string
	Ollama         OllamaConfig
	OpenAI         OpenAIConfig
	RequestTimeout time.Duration
	OllamaTimeout  time.Duration
	OpenAITimeout  time.Duration
	StartupTimeout time.Duration
	PowerInterval  time.Duration
}

// OllamaConfig stores upstream Ollama settings.
type OllamaConfig struct {
	BaseURL string
}

// OpenAIConfig stores upstream OpenAI settings.
type OpenAIConfig struct {
	BaseURL string
	APIKey  string
}

// LoadFromEnv builds config with sensible defaults and environment overrides.
func LoadFromEnv() Config {
	cfg := Config{
		ListenAddress: defaultListenAddress,
		Ollama: OllamaConfig{
			BaseURL: defaultOllamaBaseURL,
		},
		OpenAI: OpenAIConfig{
			BaseURL: defaultOpenAIBaseURL,
		},
		RequestTimeout: defaultRequestTimeout,
		OllamaTimeout:  defaultOllamaTimeout,
		OpenAITimeout:  defaultOpenAITimeout,
		StartupTimeout: defaultStartupTimeout,
		PowerInterval:  defaultPowerInterval,
	}

	if v := os.Getenv("SOLAS_LISTEN_ADDRESS"); v != "" {
		cfg.ListenAddress = v
	}
	if v := os.Getenv("SOLAS_OLLAMA_BASE_URL"); v != "" {
		cfg.Ollama.BaseURL = v
	}
	if v := os.Getenv("SOLAS_OPENAI_BASE_URL"); v != "" {
		cfg.OpenAI.BaseURL = v
	}
	if v := os.Getenv("SOLAS_OPENAI_API_KEY"); v != "" {
		cfg.OpenAI.APIKey = v
	}
	if v := os.Getenv("SOLAS_REQUEST_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RequestTimeout = d
		}
	}
	if v := os.Getenv("SOLAS_OLLAMA_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.OllamaTimeout = d
		}
	}
	if v := os.Getenv("SOLAS_OPENAI_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.OpenAITimeout = d
		}
	}
	if v := os.Getenv("SOLAS_STARTUP_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.StartupTimeout = d
		}
	}
	if v := os.Getenv("SOLAS_POWER_SAMPLE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PowerInterval = d
		}
	}

	return cfg
}

// Validate verifies config values are syntactically valid.
func Validate(cfg Config) error {
	if _, err := url.ParseRequestURI(cfg.Ollama.BaseURL); err != nil {
		return fmt.Errorf("invalid ollama base url: %w", err)
	}
	if _, err := url.ParseRequestURI(cfg.OpenAI.BaseURL); err != nil {
		return fmt.Errorf("invalid openai base url: %w", err)
	}
	if _, err := net.ResolveTCPAddr("tcp", cfg.ListenAddress); err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}
	if cfg.RequestTimeout <= 0 || cfg.OllamaTimeout <= 0 || cfg.OpenAITimeout <= 0 || cfg.StartupTimeout <= 0 || cfg.PowerInterval <= 0 {
		return fmt.Errorf("timeouts must be greater than zero")
	}
	return nil
}
