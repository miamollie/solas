APP_NAME ?= solas
BIN_DIR ?= bin
BIN_PATH ?= $(BIN_DIR)/$(APP_NAME)
GO ?= go

LISTEN_ADDRESS ?= :8000
OLLAMA_BASE_URL ?= http://ollama:11434
REQUEST_TIMEOUT ?= 60s
OLLAMA_TIMEOUT ?= 60s
STARTUP_TIMEOUT ?= 10s

DOCKER_IMAGE ?= solas

.PHONY: help build run test lint tidy clean docker-build docker-run docker-run-local stack-up stack-down stack-status stack-logs install-hooks

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Available targets:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build solas binary
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_PATH) ./cmd/solas

run: build ## Run solas 
	SOLAS_LISTEN_ADDRESS=$(LISTEN_ADDRESS) \
	SOLAS_OLLAMA_BASE_URL=$(OLLAMA_BASE_URL) \
	SOLAS_REQUEST_TIMEOUT=$(REQUEST_TIMEOUT) \
	SOLAS_OLLAMA_TIMEOUT=$(OLLAMA_TIMEOUT) \
	SOLAS_STARTUP_TIMEOUT=$(STARTUP_TIMEOUT) \
	./$(BIN_PATH)

test: ## Run all Go tests
	$(GO) test ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

tidy: ## Tidy Go modules
	$(GO) mod tidy

install-hooks: ## Install git hooks (e.g., pre-push lint)
	git config core.hooksPath .githooks

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Run Docker image (expects Ollama reachable from container)
	docker run --rm -p 8000:8000 $(DOCKER_IMAGE)

stack-up: ## Bring up Solas + Prometheus + Grafana stack (uses solas-stack/docker-compose.yml)
	docker compose -f solas-stack/docker-compose.yml up -d --build

stack-down: ## Bring down Solas + Prometheus + Grafana stack
	docker compose -f solas-stack/docker-compose.yml down

stack-status: ## Show stack container status
	docker compose -f solas-stack/docker-compose.yml ps

stack-logs: ## Show stack logs (pass args via ARGS, e.g. ARGS='-f solas')
	docker compose -f solas-stack/docker-compose.yml logs $(ARGS)
