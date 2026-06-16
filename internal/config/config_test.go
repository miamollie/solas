package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	t.Setenv("GREENOPSD_LISTEN_ADDRESS", "")
	t.Setenv("GREENOPSD_OLLAMA_BASE_URL", "")

	cfg := LoadFromEnv()
	if cfg.ListenAddress != defaultListenAddress {
		t.Fatalf("expected default listen address %q, got %q", defaultListenAddress, cfg.ListenAddress)
	}
	if cfg.Ollama.BaseURL != defaultOllamaBaseURL {
		t.Fatalf("expected default ollama base url %q, got %q", defaultOllamaBaseURL, cfg.Ollama.BaseURL)
	}
	if cfg.RequestTimeout != defaultRequestTimeout || cfg.OllamaTimeout != defaultOllamaTimeout || cfg.StartupTimeout != defaultStartupTimeout {
		t.Fatalf("expected default timeouts to be loaded")
	}
}

func TestLoadFromEnvOverrides(t *testing.T) {
	t.Setenv("GREENOPSD_LISTEN_ADDRESS", ":9000")
	t.Setenv("GREENOPSD_OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("GREENOPSD_REQUEST_TIMEOUT", "30s")
	t.Setenv("GREENOPSD_OLLAMA_TIMEOUT", "15s")
	t.Setenv("GREENOPSD_STARTUP_TIMEOUT", "7s")

	cfg := LoadFromEnv()
	if cfg.ListenAddress != ":9000" {
		t.Fatalf("expected overridden listen address, got %q", cfg.ListenAddress)
	}
	if cfg.Ollama.BaseURL != "http://ollama:11434" {
		t.Fatalf("expected overridden ollama url, got %q", cfg.Ollama.BaseURL)
	}
	if cfg.RequestTimeout != 30*time.Second || cfg.OllamaTimeout != 15*time.Second || cfg.StartupTimeout != 7*time.Second {
		t.Fatalf("expected timeout overrides to be loaded")
	}
}

func TestValidateFailsForInvalidURL(t *testing.T) {
	err := Validate(Config{ListenAddress: ":8000", Ollama: OllamaConfig{BaseURL: "::://bad-url"}})
	if err == nil {
		t.Fatalf("expected validation error for invalid URL")
	}
}

func TestValidateFailsForInvalidListenAddress(t *testing.T) {
	err := Validate(Config{ListenAddress: "127.0.0.1:notaport", Ollama: OllamaConfig{BaseURL: "http://127.0.0.1:11434"}})
	if err == nil {
		t.Fatalf("expected validation error for invalid listen address")
	}
}
