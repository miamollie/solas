# Configuration

`greenopsd` uses sensible defaults and supports environment overrides.

## Defaults

```yaml
listen_address: ":8000"

ollama:
  base_url: "http://127.0.0.1:11434"
```

## Environment Variables

- `GREENOPSD_LISTEN_ADDRESS`
- `GREENOPSD_OLLAMA_BASE_URL`
- `GREENOPSD_REQUEST_TIMEOUT` (default `60s`)
- `GREENOPSD_OLLAMA_TIMEOUT` (default `60s`)
- `GREENOPSD_STARTUP_TIMEOUT` (default `10s`)
