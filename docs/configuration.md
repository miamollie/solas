# Configuration

`greenopsd` uses sensible defaults and supports environment overrides.

## Defaults

```yaml
listen_address: ":8000"

ollama:
  base_url: "http://localhost:11434"
```

## Environment Variables

- `GREENOPSD_LISTEN_ADDRESS`
- `GREENOPSD_OLLAMA_BASE_URL`
