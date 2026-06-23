package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	t.Setenv("SOLAS_LISTEN_ADDRESS", "")
	t.Setenv("SOLAS_OLLAMA_BASE_URL", "")
	t.Setenv("SOLAS_OPENAI_BASE_URL", "")
	t.Setenv("SOLAS_OPENAI_API_KEY", "")

	cfg := LoadFromEnv()
	if cfg.ListenAddress != defaultListenAddress {
		t.Fatalf("expected default listen address %q, got %q", defaultListenAddress, cfg.ListenAddress)
	}
	if cfg.Ollama.BaseURL != defaultOllamaBaseURL {
		t.Fatalf("expected default ollama base url %q, got %q", defaultOllamaBaseURL, cfg.Ollama.BaseURL)
	}
	if cfg.OpenAI.BaseURL != defaultOpenAIBaseURL {
		t.Fatalf("expected default openai base url %q, got %q", defaultOpenAIBaseURL, cfg.OpenAI.BaseURL)
	}
	if cfg.RequestTimeout != defaultRequestTimeout || cfg.OllamaTimeout != defaultOllamaTimeout || cfg.OpenAITimeout != defaultOpenAITimeout || cfg.StartupTimeout != defaultStartupTimeout {
		t.Fatalf("expected default timeouts to be loaded")
	}
}

func TestLoadFromEnvOverrides(t *testing.T) {
	t.Setenv("SOLAS_LISTEN_ADDRESS", ":9000")
	t.Setenv("SOLAS_OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("SOLAS_OPENAI_BASE_URL", "http://openai-proxy:8080")
	t.Setenv("SOLAS_OPENAI_API_KEY", "test-key")
	t.Setenv("SOLAS_REQUEST_TIMEOUT", "30s")
	t.Setenv("SOLAS_OLLAMA_TIMEOUT", "15s")
	t.Setenv("SOLAS_OPENAI_TIMEOUT", "12s")
	t.Setenv("SOLAS_STARTUP_TIMEOUT", "7s")

	cfg := LoadFromEnv()
	if cfg.ListenAddress != ":9000" {
		t.Fatalf("expected overridden listen address, got %q", cfg.ListenAddress)
	}
	if cfg.Ollama.BaseURL != "http://ollama:11434" {
		t.Fatalf("expected overridden ollama url, got %q", cfg.Ollama.BaseURL)
	}
	if cfg.OpenAI.BaseURL != "http://openai-proxy:8080" || cfg.OpenAI.APIKey != "test-key" {
		t.Fatalf("expected overridden openai config")
	}
	if cfg.RequestTimeout != 30*time.Second || cfg.OllamaTimeout != 15*time.Second || cfg.OpenAITimeout != 12*time.Second || cfg.StartupTimeout != 7*time.Second {
		t.Fatalf("expected timeout overrides to be loaded")
	}
}

func TestValidateFailsForInvalidURL(t *testing.T) {
	err := Validate(Config{ListenAddress: ":8000", Ollama: OllamaConfig{BaseURL: "::://bad-url"}, OpenAI: OpenAIConfig{BaseURL: "https://api.openai.com"}, RequestTimeout: time.Second, OllamaTimeout: time.Second, OpenAITimeout: time.Second, StartupTimeout: time.Second})
	if err == nil {
		t.Fatalf("expected validation error for invalid URL")
	}
}

func TestValidateFailsForInvalidOpenAIURL(t *testing.T) {
	err := Validate(Config{ListenAddress: ":8000", Ollama: OllamaConfig{BaseURL: "http://127.0.0.1:11434"}, OpenAI: OpenAIConfig{BaseURL: "::://bad-url"}, RequestTimeout: time.Second, OllamaTimeout: time.Second, OpenAITimeout: time.Second, StartupTimeout: time.Second})
	if err == nil {
		t.Fatalf("expected validation error for invalid openai URL")
	}
}

func TestValidateFailsForInvalidListenAddress(t *testing.T) {
	err := Validate(Config{ListenAddress: "127.0.0.1:notaport", Ollama: OllamaConfig{BaseURL: "http://127.0.0.1:11434"}, OpenAI: OpenAIConfig{BaseURL: "https://api.openai.com"}, RequestTimeout: time.Second, OllamaTimeout: time.Second, OpenAITimeout: time.Second, StartupTimeout: time.Second})
	if err == nil {
		t.Fatalf("expected validation error for invalid listen address")
	}
}
