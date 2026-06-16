package main

import (
	"fmt"

	"github.com/miamollie/greenops-local-llm/internal/config"
)

func main() {
	cfg := config.LoadFromEnv()
	fmt.Printf("greenopsd listening on %s (ollama: %s)\n", cfg.ListenAddress, cfg.Ollama.BaseURL)
}
