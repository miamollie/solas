# Solas stack: Ollama on host + OpenWebUI + GreenOps Observabillity layer

This stack runs:

- `solas` on `http://localhost:8000`
- Open WebUI on `http://localhost:3000`
- Prometheus on `http://localhost:9090`
- Grafana on `http://localhost:3001`

Ollama stays on your host machine so it can use your local GPU/accelerator.

## Prerequisites

1. Install and start Ollama on host.
2. Pull at least one model in Ollama.
3. Install Docker Desktop.

Quick check:

```bash
curl http://127.0.0.1:11434/api/tags
```

## Bring stack up

From repository root:

```bash
make stack-up
```

or directly via CLI:

```bash
go build -o bin/solas ./cmd/solas
./bin/solas up
```

The `solas up` command checks that Ollama is reachable on host first, then runs Docker Compose from `solas-stack/docker-compose.yml`.

Use Open WebUI with:

- OpenAI base URL: `http://host.docker.internal:8000/v1`
- API key: `solas`

## Bring stack down

```bash
make stack-down
```

or:

```bash
./bin/solas down
```

## Check status

```bash
make stack-status
```

or:

```bash
./bin/solas status
```

This shows Docker Compose service state and whether host Ollama is reachable.

## View logs

```bash
make stack-logs
```

or:

```bash
./bin/solas logs
```

Useful variants:

- Follow all logs: `./bin/solas logs -f`
- One service: `./bin/solas logs -f solas`
- Make target with args: `make stack-logs ARGS='-f open-webui'`

## Notes

- Grafana default credentials are `admin` / `admin`.
- A pre-provisioned dashboard named `Solas GreenOps Overview` is loaded automatically under the `Solas` folder.
- Prometheus is preconfigured to scrape Solas at `solas:8000` inside Docker network.
- Running Ollama inside Docker on macOS is typically slower and does not use host GPU acceleration effectively.
