# Client Attribution

For each OpenAI-compatible request, solas captures caller metadata:

- User-Agent (from HTTP header)
- Remote IP (parsed from connection address)
- Optional client name from `X-GreenOps-Client`

These are exported via Prometheus as labels on:

- `solas_client_requests_total`

Example values for `X-GreenOps-Client`:

- `continue`
- `cline`
- `open-webui`
