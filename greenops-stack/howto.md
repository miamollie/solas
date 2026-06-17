# How to run an LLM locally with GreenOps observability

1. Install Ollama
2. Add some models
3. GreenOpsd using docker ( default is to run on docker internal 8000)
4. Open WebUI to have some chats, you will need to log in

```
docker run -d \
  --name open-webui \
  -p 3000:8080 \
  -e OPENAI_API_BASE_URL=http://host.docker.internal:8000/v1 \
  -e OPENAI_API_KEY=local-dev-key \
  -v open-webui:/app/backend/data \
  ghcr.io/open-webui/open-webui:main
```

5. Run prometheus 
(At this point, run it from the greenops route directory)

```
docker run -d \
  --name prometheus \
  -p 9090:9090 \
  -v prometheus-data:/prometheus \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

6. Run grafana so you can see what's going on

```
docker run -d \
  --name grafana \
  -p 3001:3000 \
  -e GF_SECURITY_ADMIN_USER=admin \
  -e GF_SECURITY_ADMIN_PASSWORD=admin \
  -v grafana-data:/var/lib/grafana \
  grafana/grafana:latest

```
- might need to configure it to look at the right endpoint (tbd: http://host.docker.internal:9090)

### Now you've got

                ┌──────────────────┐
                │   Open WebUI     │
                │ http://localhost │
                └────────┬─────────┘
                         │
                         ▼
            greenSolasLLMOps (your gateway)
                         │
                         ▼
                     Ollama

                         ▲
                         │
                ┌────────┴────────┐
                │    Grafana      │
                │ localhost:3001  │
                └──────────────────┘
