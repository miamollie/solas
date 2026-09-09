package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	t.Setenv("SOLAS_LISTEN_ADDRESS", "")
	t.Setenv("SOLAS_OLLAMA_BASE_URL", "")
	t.Setenv("SOLAS_OLLAMA_CONTAINER_NAME", "")
	// process profile mode env removed; default behavior is container-only.

	cfg := LoadFromEnv()
	if cfg.ListenAddress != defaultListenAddress {
		t.Fatalf("expected default listen address %q, got %q", defaultListenAddress, cfg.ListenAddress)
	}
	if cfg.Ollama.BaseURL != defaultOllamaBaseURL {
		t.Fatalf("expected default ollama base url %q, got %q", defaultOllamaBaseURL, cfg.Ollama.BaseURL)
	}
	if cfg.Ollama.ContainerName != "" {
		t.Fatalf("expected default ollama container name to be empty")
	}
	// process profiling removed; container-only operation.
	if cfg.RequestTimeout != defaultRequestTimeout || cfg.OllamaTimeout != defaultOllamaTimeout || cfg.StartupTimeout != defaultStartupTimeout || cfg.PowerInterval != defaultPowerInterval {
		t.Fatalf("expected default timeouts to be loaded")
	}
}

func TestLoadFromEnvOverrides(t *testing.T) {
	t.Setenv("SOLAS_LISTEN_ADDRESS", ":9000")
	t.Setenv("SOLAS_OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("SOLAS_OLLAMA_CONTAINER_NAME", "solas-ollama-1")
	// process profiling env removed; ignored.
	t.Setenv("SOLAS_REQUEST_TIMEOUT", "30s")
	t.Setenv("SOLAS_OLLAMA_TIMEOUT", "15s")
	t.Setenv("SOLAS_STARTUP_TIMEOUT", "7s")
	t.Setenv("SOLAS_POWER_SAMPLE_INTERVAL", "3s")

	cfg := LoadFromEnv()
	if cfg.ListenAddress != ":9000" {
		t.Fatalf("expected overridden listen address, got %q", cfg.ListenAddress)
	}
	if cfg.Ollama.BaseURL != "http://ollama:11434" {
		t.Fatalf("expected overridden ollama url, got %q", cfg.Ollama.BaseURL)
	}
	if cfg.Ollama.ContainerName != "solas-ollama-1" {
		t.Fatalf("expected overridden ollama container name, got %q", cfg.Ollama.ContainerName)
	}
	// process profiling removed; container-only operation.
	if cfg.RequestTimeout != 30*time.Second || cfg.OllamaTimeout != 15*time.Second || cfg.StartupTimeout != 7*time.Second || cfg.PowerInterval != 3*time.Second {
		t.Fatalf("expected timeout overrides to be loaded")
	}
}

func TestValidateFailsForInvalidURL(t *testing.T) {
	err := Validate(Config{ListenAddress: ":8000", Ollama: OllamaConfig{BaseURL: "::://bad-url"}, RequestTimeout: time.Second, OllamaTimeout: time.Second, StartupTimeout: time.Second, PowerInterval: time.Second})
	if err == nil {
		t.Fatalf("expected validation error for invalid URL")
	}
}

func TestValidateFailsForInvalidListenAddress(t *testing.T) {
	err := Validate(Config{ListenAddress: "127.0.0.1:notaport", Ollama: OllamaConfig{BaseURL: "http://127.0.0.1:11434"}, RequestTimeout: time.Second, OllamaTimeout: time.Second, StartupTimeout: time.Second, PowerInterval: time.Second})
	if err == nil {
		t.Fatalf("expected validation error for invalid listen address")
	}
}

// process profiling modes removed; no tests for process mode.
