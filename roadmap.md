# Solas Energy Attribution Roadmap

## Purpose

Track delivery progress from the current targeted implementation toward a robust, cross-platform, and model-attribution-focused system.

## Current State (as of 2026-06-17)

### Completed

- [x] OpenAI-compatible chat endpoint support (including SSE streaming)
- [ ] Request/token metrics foundation - capture user message vs system prompts
- [x] Fix streaming issue
- [x] Initial power package scaffolding in `internal/power`
- [x] macOS `powermetrics` collector implementation
- [x] Collector parsing tests (CPU/GPU watts and mW)

### In Progress

- [ ] Continuous power sampling loop
- [ ] Power metrics export (current watts + total joules)
- [ ] Request-window registry for attribution
- [ ] Request energy attribution engine

### Not Started

- [ ] Process-level Ollama attribution
- [ ] Model efficiency ranking outputs
- [ ] Attribution confidence scoring

---

## Target State (Milestone 1 MVP)

Deliver machine-level energy approximation for local LLM usage, correlated by request windows.

### MVP Questions to Answer

- How much energy did local LLM usage consume today?
- Which model consumed more energy?
- What is estimated energy per request?
- What is estimated energy per token?

### MVP Deliverables

- [ ] Current machine power (CPU/GPU/total)
- [ ] Total machine energy counter (joules)
- [ ] Estimated energy per request
- [ ] Estimated energy per model
- [ ] Tokens-per-joule and joules-per-token metrics

---

## Execution Plan

### 1) Sampling and Power Pipeline

- [ ] Add a background sampler service in `internal/power`
- [ ] Collect and buffer timestamped samples at fixed interval
- [ ] Keep bounded in-memory history for attribution windows
- [ ] Surface collector health and last error state

### 2) Prometheus Metrics for Power and Energy

- [ ] Add `solas_cpu_power_watts` gauge
- [ ] Add `solas_gpu_power_watts` gauge
- [ ] Add `solas_total_power_watts` gauge
- [ ] Add `solas_energy_joules_total` counter
- [ ] Add `solas_power_collection_available` gauge

### 3) Request Window Tracking

- [ ] Create `internal/attribution`
- [ ] Record request start/end timestamps
- [ ] Record endpoint/model/request ID
- [ ] Record token usage per request
- [ ] Add concurrency-safe registry and cleanup

### 4) Attribution Engine (Window Correlation)

- [ ] Correlate request windows with sampled power
- [ ] Compute average power over each request window
- [ ] Compute request energy in joules
- [ ] Aggregate by model and endpoint

### 5) Request Attribution Metrics

- [ ] Add `solas_request_energy_joules_total`
- [ ] Add `solas_request_tokens_total`
- [ ] Add `solas_request_energy_joules` histogram
- [ ] Add `solas_request_duration_seconds` histogram

### 6) Dashboard and Docs

- [ ] Commit Grafana dashboard JSON
- [ ] Add metrics reference docs
- [ ] Add setup guide for local observability stack
- [ ] Add macOS permissions and troubleshooting guide

---

## Generic Architecture Improvements (from targeted to reusable)

Current code is intentionally targeted. These improvements move it to a generic architecture:

### Interface-Driven Collector Design

- [ ] Introduce `PowerCollector` interface with explicit capabilities
- [ ] Introduce `SampleStore` interface for retention/query strategy
- [ ] Introduce `AttributionEngine` interface for pluggable models
- [ ] Introduce `Clock` interface for deterministic tests

### Dependency Injection and Composition

- [ ] Compose services in a dedicated bootstrap package
- [ ] Keep HTTP handlers unaware of collector implementation details
- [ ] Isolate metrics emission behind adapter interfaces

### Configuration Generalization

- [ ] Add collector interval and retention configs
- [ ] Add feature flags for attribution modes
- [ ] Add environment-specific defaults and overrides

### Testing Improvements

- [ ] Add end-to-end attribution tests with fake sampler
- [ ] Add fixture-based parser tests across collector backends
- [ ] Add race-condition tests for registry/sampler concurrency

---

## Stretch Goals

### Cross-Platform Support (not macOS-only)

- [ ] Define OS-agnostic collector interfaces and registration
- [ ] Add Linux collector implementation
- [ ] Add Windows collector implementation
- [ ] Add capability detection and runtime backend selection
- [ ] Add per-backend feature/capability metrics

### Generic Backends and Extensibility

- [ ] Support multiple collection backends per OS (CLI/API/vendor)
- [ ] Allow backend priority and fallback chains
- [ ] Separate parsing layer from execution layer
- [ ] Add backend plugin contract for future integrations

### Attribution Quality Enhancements

- [ ] Idle-baseline subtraction
- [ ] Concurrent-request disambiguation strategy
- [ ] Confidence scoring with quality signals
- [ ] Error bounds for reported energy estimates

---

## Definition of Done (MVP)

- [ ] Users can query energy consumed over a time window
- [ ] Users can compare energy by model
- [ ] Users can view per-request energy and efficiency metrics
- [ ] Metrics and dashboard are reproducible via documented setup
