# solas

`solas` is a lightweight Go service that provides an OpenAI-compatible facade in front of Ollama and exports Prometheus metrics. The goals is to have a proxy layer enabling easier GreenOps metrics gathering and visualisation.

## How it works

<!-- TODO -->

## Logging

`solas` uses structured `slog` logging in JSON format and assigns request IDs via `X-Request-ID`.

## Project Layout

- `cmd/solas`: binary entrypoint
- `internal/`: private application packages
- `pkg/`: public/shared packages (if needed)

## Docker

Build and run:

```bash
docker build -t solas .
docker run --rm -p 8000:8000 solas
```

Container health checks probe both `/health` and `/ready`.
