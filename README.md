# solas

`solas` is a lightweight Go service that provides an OpenAI-compatible facade in front of Ollama and exports Prometheus metrics. The goals is to have a proxy layer enabling easier GreenOps metrics gathering and visualisation.

Starting with token counts, how far can we responsibly follow that number down to the physical machine?

## How it works

Solas sits between your LLM clients (Open WebUI, Continue, Cline, …) and a locally-running Ollama instance. Every request arrives at a standard OpenAI-compatible endpoint, Solas proxies it to Ollama, and returns an OpenAI-shaped response. Along the way it measures tokens per request.

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


## Debugging

Use the requests.http file to debug from your host to the docker network.

Use the following command to debug service to service requests within the docker network. You can "exec" into any container in the network.

```bash
docker exec -it solas-stack-open-webui-1 sh -lc 'curl -sS http://solas:8000/v1/models'


docker exec -it solas-stack-open-webui-1 sh -lc \
'curl -sS http://solas:8000/v1/models \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer solas"'
```
