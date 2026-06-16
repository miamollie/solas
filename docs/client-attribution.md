# Client Attribution

For each OpenAI-compatible request, greenopsd captures caller metadata:

- User-Agent (from HTTP header)
- Remote IP (parsed from connection address)
- Optional client name from `X-GreenOps-Client`

These are exported via Prometheus as labels on:

- `greenops_client_requests_total`

Example values for `X-GreenOps-Client`:

- `continue`
- `cline`
- `open-webui`
