# Energy Attribution

Solas now exposes host power, in-flight LLM concurrency, and best-effort local LLM process tracking so you can correlate energy spikes with query overlap.

## What is reported

Request and token context:

- `solas_llm_inflight_requests{provider}`
- `solas_input_total_tokens_total{model}`
- `solas_input_user_tokens_total{model}`
- `solas_input_accumulated_tokens_total{model}`
- `solas_input_overhead_tokens_total{model}`
- `solas_output_tokens_total{model}`

Power telemetry:

- `solas_power_cpu_watts`
- `solas_power_gpu_watts`
- `solas_power_total_watts`
- `solas_power_collector_healthy`

Process telemetry (local providers only):

- `solas_llm_process_pid{provider}`
- `solas_llm_process_cpu_percent{provider}`
- `solas_llm_process_rss_bytes{provider}`

Process sampling supports two modes:

- `device` mode (default): resolve local loopback provider PIDs and sample via host process tools (`lsof`, `ps`).
- `container` mode: sample provider container CPU/memory from `docker stats`.

## How PID tracking works

Solas resolves a provider PID when the provider base URL is local loopback (`localhost`, `127.0.0.1`, `::1`) by mapping the provider port to the listening process.

For `container` mode, Solas resolves provider containers via `SOLAS_OLLAMA_CONTAINER_NAME` (recommended) or infers from the provider hostname when possible.


## Sampling controls

Power/process sampling interval is configured with:

- `SOLAS_POWER_SAMPLE_INTERVAL` (default: `5s`)

## Correlation workflow

1. Capture a baseline idle window.
2. Capture local interactive window (for example Chrome and VS Code only).
3. Run LLM traffic and compare against baseline using:
   - in-flight query overlap (`solas_llm_inflight_requests`)
   - power spikes (`solas_power_total_watts`)
   - local LLM process load (`solas_llm_process_cpu_percent`, `solas_llm_process_rss_bytes`)

## Notes

- On macOS, host power collection is based on `powermetrics` output parsing.
- If host power collection fails, `solas_power_collector_healthy` is set to `0`.
- `container` mode still reports provider CPU/memory only; host power watts remain sourced from host collector.
