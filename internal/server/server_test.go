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

func (f fakeLLMClient) GetVersion(_ context.Context) (any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return map[string]any{"version": "0.9.0"}, nil
}

func (f fakeLLMClient) GetRunningModels(_ context.Context) (any, error) {
	if f.err != nil {
		return nil, f.err
	}
	return map[string]any{"models": []any{map[string]any{"name": "qwen3:32b"}}}, nil
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
	return New(logger, client, metrics.New())
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
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

func TestModelsEndpointOpenAIContract(t *testing.T) {
	s := newTestServer(fakeLLMClient{modelsAny: ollama.OllamaTagsResponse{Models: []ollama.OllamaTagModel{{Name: "qwen3:32b"}}}})
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload chat.OpenAIModelsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.Object != "list" || len(payload.Data) != 1 || payload.Data[0].ID != "qwen3:32b" {
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
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeLLMClient{chatResp: chat.Response{
		Model: "qwen3:32b", Message: chat.Message{Role: "assistant", Content: "ok"}, PromptTokens: 4, CompletionTokens: 6,
	}, modelsAny: ollama.OllamaTagsResponse{}}, m)
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"system","content":"ctx"},{"role":"user","content":"hello"},{"role":"assistant","content":"ok"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
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
}

func TestOpenAIChatEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{chatResp: chat.Response{
		Model:            "qwen3:32b",
		Message:          chat.Message{Role: "assistant", Content: "hi there"},
		PromptTokens:     4,
		CompletionTokens: 6,
	}})
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var payload chat.OpenAIChatCompletionsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Model != "qwen3:32b" || len(payload.Choices) != 1 || payload.Usage.TotalTokens != 10 {
		t.Fatalf("unexpected response payload: %s", rr.Body.String())
	}
}

func TestOpenAIChatStreamingEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{stream: []chat.StreamChunk{
		{Model: "qwen3:32b", Message: chat.Message{Role: "assistant", Content: "hel"}, Done: false},
		{Model: "qwen3:32b", Message: chat.Message{Role: "assistant", Content: "lo"}, Done: true, PromptTokens: 3, CompletionTokens: 5},
	}})
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream content type, got %q", got)
	}
	bodyText := rr.Body.String()
	if !strings.Contains(bodyText, `"object":"chat.completion.chunk"`) {
		t.Fatalf("expected chunk payload, got %s", bodyText)
	}
	if !strings.Contains(bodyText, `"content":"hel"`) || !strings.Contains(bodyText, `"content":"lo"`) {
		t.Fatalf("expected streamed content, got %s", bodyText)
	}
	if !strings.Contains(bodyText, "data: [DONE]") {
		t.Fatalf("expected done sentinel, got %s", bodyText)
	}
}

func TestLegacyEndpointsRemoved(t *testing.T) {
	s := newTestServer(fakeLLMClient{})
	for _, path := range []string{"/ollama/api/chat", "/ollama/api/tags", "/openai/v1/chat/completions"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for %s, got %d", path, rr.Code)
		}
	}
}
