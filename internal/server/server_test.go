package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/miamollie/greenops-local-llm/internal/metrics"
	"github.com/miamollie/greenops-local-llm/internal/ollama"
)

type fakeReadyChecker struct {
	err    error
	models []ollama.TagModel
	chat   ollama.ChatResponse
	stream []ollama.ChatResponse
}

type captureModelChecker struct {
	receivedModel string
}

func (c *captureModelChecker) IsReachable(_ context.Context) error {
	return nil
}

func (c *captureModelChecker) ListModels(_ context.Context) ([]ollama.TagModel, error) {
	return nil, nil
}

func (c *captureModelChecker) Chat(_ context.Context, reqBody ollama.ChatRequest) (ollama.ChatResponse, error) {
	c.receivedModel = reqBody.Model
	return ollama.ChatResponse{Model: reqBody.Model, Message: ollama.ChatMessage{Role: "assistant", Content: "ok"}}, nil
}

func (c *captureModelChecker) ChatStream(_ context.Context, reqBody ollama.ChatRequest) (io.ReadCloser, error) {
	c.receivedModel = reqBody.Model
	return io.NopCloser(strings.NewReader("")), nil
}

func (f fakeReadyChecker) IsReachable(_ context.Context) error {
	return f.err
}

func (f fakeReadyChecker) ListModels(_ context.Context) ([]ollama.TagModel, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.models, nil
}

func (f fakeReadyChecker) Chat(_ context.Context, reqBody ollama.ChatRequest) (ollama.ChatResponse, error) {
	if f.err != nil {
		return ollama.ChatResponse{}, f.err
	}
	if reqBody.Stream {
		return ollama.ChatResponse{}, errors.New("expected non-streaming")
	}
	return f.chat, nil
}

func (f fakeReadyChecker) ChatStream(_ context.Context, reqBody ollama.ChatRequest) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	if !reqBody.Stream {
		return nil, errors.New("expected streaming")
	}
	lines := make([]string, 0, len(f.stream))
	for _, c := range f.stream {
		raw, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		lines = append(lines, string(raw))
	}
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n"))), nil
}

func TestHealthEndpoint(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{}, metrics.New())
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected status=ok, got %q", payload["status"])
	}
}

func TestReadyEndpointHealthy(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{}, metrics.New())
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestReadyEndpointUnavailable(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{err: errors.New("down")}, metrics.New())
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestModelsEndpoint(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{models: []ollama.TagModel{{Name: "qwen3:32b"}}}, metrics.New())
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.Object != "list" || len(payload.Data) != 1 || payload.Data[0].ID != "qwen3:32b" {
		t.Fatalf("unexpected payload: %s", rr.Body.String())
	}
}

func TestChatCompletionsEndpoint(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{chat: ollama.ChatResponse{
		Model:           "qwen3:32b",
		Message:         ollama.ChatMessage{Role: "assistant", Content: "hi there"},
		PromptEvalCount: 4,
		EvalCount:       6,
	}}, metrics.New())
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload struct {
		Model string `json:"model"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Model != "qwen3:32b" || payload.Usage.TotalTokens != 10 {
		t.Fatalf("unexpected response payload: %s", rr.Body.String())
	}
}

func TestChatCompletionsStreamingSSE(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{stream: []ollama.ChatResponse{
		{Model: "qwen3:32b", Message: ollama.ChatMessage{Role: "assistant", Content: "hel"}, Done: false},
		{Model: "qwen3:32b", Message: ollama.ChatMessage{Role: "assistant", Content: "lo"}, Done: true, PromptEvalCount: 3, EvalCount: 5},
	}}, metrics.New())
	body := []byte(`{"model":"qwen3:32b","messages":[],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", got)
	}
	bodyText := rr.Body.String()
	if !strings.Contains(bodyText, `"object":"chat.completion.chunk"`) {
		t.Fatalf("expected chat completion chunks in SSE output, got %s", bodyText)
	}
	if !strings.Contains(bodyText, `"content":"hel"`) || !strings.Contains(bodyText, `"content":"lo"`) {
		t.Fatalf("expected streamed content chunks in SSE output, got %s", bodyText)
	}
	if !strings.Contains(bodyText, `"finish_reason":"stop"`) {
		t.Fatalf("expected terminal finish reason in SSE output, got %s", bodyText)
	}
	if !strings.Contains(bodyText, "data: [DONE]") {
		t.Fatalf("expected [DONE] in SSE output, got %s", bodyText)
	}
}

func TestChatCompletionsModelPassthrough(t *testing.T) {
	checker := &captureModelChecker{}
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), checker, metrics.New())
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if checker.receivedModel != "qwen3:32b" {
		t.Fatalf("expected passthrough model qwen3:32b, got %q", checker.receivedModel)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{}, metrics.New())
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "go_gc_duration_seconds") {
		t.Fatalf("expected prometheus output")
	}
}

func TestRequestMetricIncremented(t *testing.T) {
	m := metrics.New()
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeReadyChecker{chat: ollama.ChatResponse{
		Model: "qwen3:32b", Message: ollama.ChatMessage{Role: "assistant", Content: "ok"}, PromptEvalCount: 4, EvalCount: 6,
	}}, m)
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(metricsRR, metricsReq)

	if !strings.Contains(metricsRR.Body.String(), `greenops_requests_total{model="qwen3:32b",status="`+strconv.Itoa(http.StatusOK)+`"} 1`) {
		t.Fatalf("expected request counter in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `greenops_request_duration_seconds_count{model="qwen3:32b"} 1`) {
		t.Fatalf("expected duration histogram count in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `greenops_prompt_tokens_total{model="qwen3:32b"} 4`) {
		t.Fatalf("expected prompt token metric in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `greenops_completion_tokens_total{model="qwen3:32b"} 6`) {
		t.Fatalf("expected completion token metric in metrics output, got: %s", metricsRR.Body.String())
	}
}
