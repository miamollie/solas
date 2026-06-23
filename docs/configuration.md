# Configuration

`solas` uses sensible defaults and supports environment overrides.

## Defaults

```yaml
listen_address: ":8000"

ollama:
  base_url: "http://127.0.0.1:11434"
```

## Environment Variables

- `SOLAS_LISTEN_ADDRESS`
- `SOLAS_OLLAMA_BASE_URL`
- `SOLAS_REQUEST_TIMEOUT` (default `60s`)
- `SOLAS_OLLAMA_TIMEOUT` (default `60s`)
- `SOLAS_STARTUP_TIMEOUT` (default `10s`)
