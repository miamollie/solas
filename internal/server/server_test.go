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

	"github.com/miamollie/solas/internal/llmclients"
	"github.com/miamollie/solas/internal/metrics"
)

type fakeLLMClient struct {
	err       error
	modelsAny any
	chat      llmclients.ChatResponse
	stream    []llmclients.StreamChunk
}

type captureModelClient struct {
	receivedModel string
}

func (c *captureModelClient) Ready(_ context.Context) error {
	return nil
}

func (c *captureModelClient) GetModels(_ context.Context) (any, error) {
	return llmclients.OpenAIModelsResponse{Object: "list"}, nil
}

func (c *captureModelClient) Chat(_ context.Context, reqBody llmclients.ChatRequest) (llmclients.ChatResponse, error) {
	c.receivedModel = reqBody.Model
	return llmclients.ChatResponse{Model: reqBody.Model, Message: llmclients.Message{Role: "assistant", Content: "ok"}}, nil
}

func (c *captureModelClient) StreamChat(_ context.Context, reqBody llmclients.ChatRequest) (io.ReadCloser, error) {
	c.receivedModel = reqBody.Model
	return io.NopCloser(strings.NewReader("")), nil
}

func (c *captureModelClient) ParseStreamChunk(_ []byte) (llmclients.StreamChunk, error) {
	return llmclients.StreamChunk{}, nil
}

func (f fakeLLMClient) Ready(_ context.Context) error {
	return f.err
}

func (f fakeLLMClient) GetModels(_ context.Context) (any, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.modelsAny == nil {
		return llmclients.OpenAIModelsResponse{Object: "list"}, nil
	}
	return f.modelsAny, nil
}

func (f fakeLLMClient) Chat(_ context.Context, reqBody llmclients.ChatRequest) (llmclients.ChatResponse, error) {
	if f.err != nil {
		return llmclients.ChatResponse{}, f.err
	}
	if reqBody.Stream {
		return llmclients.ChatResponse{}, errors.New("expected non-streaming")
	}
	return f.chat, nil
}

func (f fakeLLMClient) StreamChat(_ context.Context, reqBody llmclients.ChatRequest) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	if !reqBody.Stream {
		return nil, errors.New("expected streaming")
	}
	lines := make([]string, 0, len(f.stream))
	for _, c := range f.stream {
		raw, err := json.Marshal(llmclients.OllamaChatResponse{
			Model:           c.Model,
			Message:         llmclients.OllamaChatMessage{Role: c.Message.Role, Content: c.Message.Content},
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

func newTestServer(openAIClient llmclients.Client, ollamaClient llmclients.Client) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(logger, openAIClient, ollamaClient, metrics.New())
}

func (f fakeLLMClient) ParseStreamChunk(line []byte) (llmclients.StreamChunk, error) {
	var chunk llmclients.OllamaChatResponse
	if err := json.Unmarshal(line, &chunk); err != nil {
		return llmclients.StreamChunk{}, err
	}
	return llmclients.StreamChunk{
		Model:            chunk.Model,
		Message:          llmclients.Message{Role: chunk.Message.Role, Content: chunk.Message.Content},
		Done:             chunk.Done,
		DoneReason:       chunk.DoneReason,
		PromptTokens:     chunk.PromptEvalCount,
		CompletionTokens: chunk.EvalCount,
	}, nil
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{}, fakeLLMClient{})
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
	s := newTestServer(fakeLLMClient{}, fakeLLMClient{})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestReadyEndpointUnavailable(t *testing.T) {
	s := newTestServer(fakeLLMClient{err: errors.New("down")}, fakeLLMClient{err: errors.New("down")})
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rr.Code)
	}
}

func TestModelsEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{modelsAny: llmclients.OpenAIModelsResponse{Object: "list", Data: []llmclients.OpenAIModelInfo{{ID: "qwen3:32b", Object: "model", OwnedBy: "ollama"}}}}, fakeLLMClient{})
	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models", nil)
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
	s := newTestServer(fakeLLMClient{chat: llmclients.ChatResponse{
		Model:            "qwen3:32b",
		Message:          llmclients.Message{Role: "assistant", Content: "hi there"},
		PromptTokens:     4,
		CompletionTokens: 6,
	}}, fakeLLMClient{})
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewReader(body))
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
	s := newTestServer(fakeLLMClient{stream: []llmclients.StreamChunk{
		{Model: "qwen3:32b", Message: llmclients.Message{Role: "assistant", Content: "hel"}, Done: false},
		{Model: "qwen3:32b", Message: llmclients.Message{Role: "assistant", Content: "lo"}, Done: true, PromptTokens: 3, CompletionTokens: 5},
	}}, fakeLLMClient{})
	body := []byte(`{"model":"qwen3:32b","messages":[],"stream":true}`)
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewReader(body))
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
	checker := &captureModelClient{}
	s := newTestServer(checker, fakeLLMClient{})
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewReader(body))
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
	s := newTestServer(fakeLLMClient{}, fakeLLMClient{})
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
	s := New(slog.New(slog.NewTextHandler(io.Discard, nil)), fakeLLMClient{chat: llmclients.ChatResponse{
		Model: "qwen3:32b", Message: llmclients.Message{Role: "assistant", Content: "ok"}, PromptTokens: 4, CompletionTokens: 6,
	}, modelsAny: llmclients.OpenAIModelsResponse{Object: "list"}}, fakeLLMClient{chat: llmclients.ChatResponse{
		Model: "qwen3:32b", Message: llmclients.Message{Role: "assistant", Content: "ok"}, PromptTokens: 4, CompletionTokens: 6,
	}, modelsAny: llmclients.OllamaTagsResponse{}}, m)
	body := []byte(`{"model":"qwen3:32b","messages":[{"role":"system","content":"ctx"},{"role":"user","content":"hello"},{"role":"assistant","content":"ok"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/openai/v1/chat/completions", bytes.NewReader(body))
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
	s := newTestServer(fakeLLMClient{}, fakeLLMClient{modelsAny: llmclients.OllamaTagsResponse{Models: []llmclients.OllamaTagModel{{Name: "qwen3:32b"}}}})
	req := httptest.NewRequest(http.MethodGet, "/ollama/api/tags", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var payload llmclients.OllamaTagsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(payload.Models) != 1 || payload.Models[0].Name != "qwen3:32b" {
		t.Fatalf("unexpected payload: %s", rr.Body.String())
	}
}

func TestOllamaChatEndpoint(t *testing.T) {
	s := newTestServer(fakeLLMClient{}, fakeLLMClient{chat: llmclients.ChatResponse{
		Model:            "qwen3:32b",
		Message:          llmclients.Message{Role: "assistant", Content: "hi there"},
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
	var payload llmclients.OllamaChatResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Model != "qwen3:32b" || payload.PromptEvalCount != 4 || payload.EvalCount != 6 {
		t.Fatalf("unexpected response payload: %s", rr.Body.String())
	}
}
