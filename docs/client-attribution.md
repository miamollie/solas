# Client Attribution

For each OpenAI-compatible request, greenopsd captures caller metadata:

- User-Agent (from HTTP header)
- Remote IP (parsed from connection address)

These are exported via Prometheus as labels on:

- `greenops_client_requests_total`
