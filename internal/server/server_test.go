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

	"github.com/miamollie/solas/internal/chat"
	"github.com/miamollie/solas/internal/metrics"
	"github.com/miamollie/solas/internal/ollama"
)

type fakeLLMClient struct {
	err       error
	modelsAny any
	chatResp  chat.Response
	stream    []chat.StreamChunk
}

type captureModelClient struct {
	receivedModel string
}

func (c *captureModelClient) Ready(_ context.Context) error {
	return nil
}

func (c *captureModelClient) GetModels(_ context.Context) (any, error) {
	return ollama.OllamaTagsResponse{}, nil
}

func (c *captureModelClient) Chat(_ context.Context, reqBody chat.Request) (chat.Response, error) {
	c.receivedModel = reqBody.Model
	return chat.Response{Model: reqBody.Model, Message: chat.Message{Role: "assistant", Content: "ok"}}, nil
}

func (c *captureModelClient) StreamChat(_ context.Context, reqBody chat.Request) (io.ReadCloser, error) {
	c.receivedModel = reqBody.Model
	return io.NopCloser(strings.NewReader("")), nil
}

func (c *captureModelClient) ParseStreamChunk(_ []byte) (chat.StreamChunk, error) {
	return chat.StreamChunk{}, nil
}

func (f fakeLLMClient) Ready(_ context.Context) error {
	return f.err
}

func (f fakeLLMClient) GetModels(_ context.Context) (any, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.modelsAny == nil {
		return ollama.OllamaTagsResponse{}, nil
	}
	return f.modelsAny, nil
}

func (f fakeLLMClient) Chat(_ context.Context, reqBody chat.Request) (chat.Response, error) {
	if f.err != nil {
		return chat.Response{}, f.err
	}
	if reqBody.Stream {
		return chat.Response{}, errors.New("expected non-streaming")
	}
	return f.chatResp, nil
}

func (f fakeLLMClient) StreamChat(_ context.Context, reqBody chat.Request) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	if !reqBody.Stream {
		return nil, errors.New("expected streaming")
	}
	lines := make([]string, 0, len(f.stream))
	for _, c := range f.stream {
		raw, err := json.Marshal(ollama.OllamaChatResponse{
			Model:           c.Model,
			Message:         ollama.OllamaChatMessage{Role: c.Message.Role, Content: c.Message.Content},
			Done:            c.Done,
			DoneReason:      c.DoneReason,
			PromptEvalCount: c.PromptTokens,
			EvalCount:       c.CompletionTokens,
		})
		if err != nil {
			return nil, err
		}
		lines = append(lines, string(raw))
	}
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n"))), nil
}

func (f fakeLLMClient) ParseStreamChunk(line []byte) (chat.StreamChunk, error) {
	var chunk ollama.OllamaChatResponse
	if err := json.Unmarshal(line, &chunk); err != nil {
		return chat.StreamChunk{}, err
	}
	return chat.StreamChunk{
		Model:            chunk.Model,
		Message:          chat.Message{Role: chunk.Message.Role, Content: chunk.Message.Content},
		Done:             chunk.Done,
		DoneReason:       chunk.DoneReason,
		PromptTokens:     chunk.PromptEvalCount,
		CompletionTokens: chunk.EvalCount,
	}, nil
}

func newTestServer(client chat.Client) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(logger, map[chat.Provider]chat.Client{chat.ProviderOllama: client}, metrics.New())
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{})
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
	s := newTestServer(fakeLLMClient{})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestReadyEndpointUnavailable(t *testing.T) {
	s := newTestServer(fakeLLMClient{err: errors.New("down")})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestModelsEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{modelsAny: ollama.OllamaTagsResponse{Models: []ollama.OllamaTagModel{{Name: "qwen3:32b"}}}})
	req := httptest.NewRequest(http.MethodGet, "/ollama/api/tags", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(payload.Models) != 1 || payload.Models[0].Name != "qwen3:32b" {
		t.Fatalf("unexpected payload: %s", rr.Body.String())
	}
}

func TestMetricsEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{})
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
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), map[chat.Provider]chat.Client{chat.ProviderOllama: fakeLLMClient{chatResp: chat.Response{
		Model: "qwen3:32b", Message: chat.Message{Role: "assistant", Content: "ok"}, PromptTokens: 4, CompletionTokens: 6,
	}, modelsAny: ollama.OllamaTagsResponse{}}}, m)
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"system","content":"ctx"},{"role":"user","content":"hello"},{"role":"assistant","content":"ok"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/ollama/api/chat", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(metricsRR, metricsReq)

	if !strings.Contains(metricsRR.Body.String(), `solas_requests_total{model="qwen3:32b",status="`+strconv.Itoa(http.StatusOK)+`"} 1`) {
		t.Fatalf("expected request counter in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `solas_request_duration_seconds_count{model="qwen3:32b"} 1`) {
		t.Fatalf("expected duration histogram count in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `solas_input_total_tokens_total{model="qwen3:32b"} 4`) {
		t.Fatalf("expected total input token metric in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `solas_input_user_tokens_total{model="qwen3:32b"} 1`) {
		t.Fatalf("expected user input token metric in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `solas_input_accumulated_tokens_total{model="qwen3:32b"} 2`) {
		t.Fatalf("expected accumulated input token metric in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `solas_input_overhead_tokens_total{model="qwen3:32b"} 1`) {
		t.Fatalf("expected overhead input token metric in metrics output, got: %s", metricsRR.Body.String())
	}
	if !strings.Contains(metricsRR.Body.String(), `solas_output_tokens_total{model="qwen3:32b"} 6`) {
		t.Fatalf("expected output token metric in metrics output, got: %s", metricsRR.Body.String())
	}
}

func TestOllamaTagsEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{modelsAny: ollama.OllamaTagsResponse{Models: []ollama.OllamaTagModel{{Name: "qwen3:32b"}}}})
	req := httptest.NewRequest(http.MethodGet, "/ollama/api/tags", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload ollama.OllamaTagsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(payload.Models) != 1 || payload.Models[0].Name != "qwen3:32b" {
		t.Fatalf("unexpected payload: %s", rr.Body.String())
	}
}

func TestOllamaChatEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{chatResp: chat.Response{
		Model:            "qwen3:32b",
		Message:          chat.Message{Role: "assistant", Content: "hi there"},
		PromptTokens:     4,
		CompletionTokens: 6,
	}})
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/ollama/api/chat", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload ollama.OllamaChatResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Model != "qwen3:32b" || payload.PromptEvalCount != 4 || payload.EvalCount != 6 {
		t.Fatalf("unexpected response payload: %s", rr.Body.String())
	}
}
