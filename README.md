# greenopsd

`greenopsd` is a lightweight Go service that provides an OpenAI-compatible facade in front of Ollama and exports Prometheus metrics.

## MVP Scope

This repository tracks MVP milestones for:

- OpenAI-compatible endpoints (`/v1/models`, `/v1/chat/completions`)
- Health and readiness probes
- Prometheus metrics
- Client attribution labels
- Docker deployment

## Logging

`greenopsd` uses structured `slog` logging in JSON format and assigns request IDs via `X-Request-ID`.

## Project Layout

- `cmd/greenopsd`: binary entrypoint
- `internal/`: private application packages
- `pkg/`: public/shared packages (if needed)

## Docker

Build and run:

```bash
docker build -t greenopsd .
docker run --rm -p 8000:8000 greenopsd
```

Container health checks probe both `/health` and `/ready`.
