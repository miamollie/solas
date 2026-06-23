APP_NAME ?= solas
BIN_DIR ?= bin
BIN_PATH ?= $(BIN_DIR)/$(APP_NAME)
GO ?= go

LISTEN_ADDRESS ?= :8000
OLLAMA_BASE_URL ?= http://127.0.0.1:11434
REQUEST_TIMEOUT ?= 60s
OLLAMA_TIMEOUT ?= 60s
STARTUP_TIMEOUT ?= 10s

DOCKER_IMAGE ?= solas

.PHONY: help build run test tidy clean docker-build docker-run docker-run-local

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Available targets:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build solas binary
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_PATH) ./cmd/solas

run: build ## Run solas with local Ollama defaults
	SOLAS_LISTEN_ADDRESS=$(LISTEN_ADDRESS) \
	SOLAS_OLLAMA_BASE_URL=$(OLLAMA_BASE_URL) \
	SOLAS_REQUEST_TIMEOUT=$(REQUEST_TIMEOUT) \
	SOLAS_OLLAMA_TIMEOUT=$(OLLAMA_TIMEOUT) \
	SOLAS_STARTUP_TIMEOUT=$(STARTUP_TIMEOUT) \
	./$(BIN_PATH)

test: ## Run all Go tests
	$(GO) test ./...

tidy: ## Tidy Go modules
	$(GO) mod tidy

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Run Docker image (expects Ollama reachable from container)
	docker run --rm -p 8000:8000 $(DOCKER_IMAGE)

docker-run-local: ## Run Docker image against host Ollama (macOS/Windows)
	docker run --rm -p 8000:8000 \
		-e SOLAS_OLLAMA_BASE_URL=http://host.docker.internal:11434 \
		$(DOCKER_IMAGE)
