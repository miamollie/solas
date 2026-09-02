# Solas stack: Ollama on host + OpenWebUI + GreenOps Observabillity layer

This stack runs:

- `solas` on `http://localhost:8000`
- Open WebUI on `http://localhost:3000`
- Prometheus on `http://localhost:9090`
- Grafana on `http://localhost:3001`
- Ollama

_Caveat: Ollama docker container suffers substantial performance degradation on MacOs due to inability to access GPU_

<!-- TODO add reference to why that is -->

## Prerequisites

1. Install docker desktop
2. Is that the only prereq?


## Bring stack up

From repository root:

```bash
make stack-up
```



Use Open WebUI with:

- OpenAI base URL: `http://host.docker.internal:8000/v1`
- API key: `solas`

## Bring stack down

```bash
make stack-down
```

## Check status

```bash
make stack-status
```

This shows Docker Compose service state and whether host Ollama is reachable.

## View logs

```bash
make stack-logs
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


## Roadmap / TODO
Tomorrow: remember why you're doing this, and go step by step from there. Don't forget this is for learning, so dead ends are valuable too

Importantly, you've lost data coming from solas into prom/grafana. In particular all the token counts etc that make this most useful as a system. 

Currently you're figuring out how to pull models in the ollama container - seemingly it will be via the webui

