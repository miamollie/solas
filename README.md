# solas

`solas` is a lightweight Go service that provides an OpenAI-compatible facade in front of Ollama and exports Prometheus metrics. The goals is to have a proxy layer enabling easier GreenOps metrics gathering and visualisation.

## How it works

Solas sits between your LLM clients (Open WebUI, Continue, Cline, …) and a locally-running Ollama instance. Every request arrives at a standard OpenAI-compatible endpoint, Solas proxies it to Ollama, and returns an OpenAI-shaped response. Along the way it measures tokens per request and continuously samples host power, so Prometheus can correlate AI traffic with energy consumption.

```
LLM client  ──POST /v1/chat/completions──►  Solas  ──POST /api/chat──►  Ollama
                                                │
                                         Prometheus /metrics
```

## API

OpenAI-compatible LLM endpoints are exposed under `/v1`; additional operational endpoints are provided for health checks and Prometheus metrics:

| Method | Path                   | Description                                   |
| ------ | ---------------------- | --------------------------------------------- |
| GET    | `/health`              | Liveness probe                                |
| GET    | `/ready`               | Readiness probe (checks Ollama connectivity)  |
| GET    | `/v1/models`           | List available Ollama models                  |
| POST   | `/v1/chat/completions` | Chat completion (streaming and non-streaming) |
| GET    | `/metrics`             | Prometheus metrics scrape endpoint            |

## Logging

`solas` uses structured `slog` logging in JSON format and assigns request IDs via `X-Request-ID`.

## Documentation

- Energy attribution: `docs/energy-attribution.md`
- Metrics reference: `docs/metrics-reference.md`

## Docker

Build and run:

```bash
docker build -t solas .
docker run --rm -p 8000:8000 solas
```

Container health checks probe both `/health` and `/ready`.
