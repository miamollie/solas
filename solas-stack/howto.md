# How to run an LLM locally with GreenOps observability

1. Install Ollama - use it on host, not from a docker container, so it can access your GPU (otherwies- [so many slow](https://chariotsolutions.com/blog/post/apple-silicon-gpus-docker-and-ollama-pick-two/) currently Docker on Mac does not see the GPU because of how virtualisation on Mac works )
2. Add some models
3. GreenOpsd using docker (default is to run on docker internal 8000)
4. Open WebUI to have some chats, you will need to log in

- Pull and run the Open WebUI docker container see [docs](https://docs.openwebui.com/getting-started/quick-start)

```
docker run -d \
  --name open-webui-solas \
  -p 3000:8080 \
  -e OPENAI_API_BASE_URL=http://host.docker.internal:8000/v1 \
  -e OPENAI_API_KEY=solas \
  -v open-webui:/app/backend/data \
  ghcr.io/open-webui/open-webui:main
```

 <!-- Optional debug step - ensure open-web-ui could read ollama directly first by ommitting the base url -->
 <!-- -v flag is the volume mount that persists chat data between runs -->
<!-- Note - even though this is ollama, you need to ensure that your openWebUI uses the OpenAI connector - note to self, is there any point in this? -->

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
   [docs](https://grafana.com/docs/grafana/latest/?utm_source=grafana_gettingstarted)

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

## Steps for VSCode connection

https://www.danielkliewer.com/blog/2024-12-19-continue.dev-ollama

Using for example Continue.dev, edit the config to point the LLM config at the exposed Solas port
