package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/miamollie/solas/internal/model"
)

// OllamaClient talks directly to Ollama's HTTP API.
type OllamaClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func New(baseURL string, httpClient *http.Client) (*OllamaClient, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse ollama base url: %w", err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OllamaClient{baseURL: u, httpClient: httpClient}, nil
}

func (c *OllamaClient) Ready(ctx context.Context) error {
	_, err := c.GetModels(ctx)
	return err
}

func (c *OllamaClient) GetModels(ctx context.Context) (any, error) {
	u := c.baseURL.JoinPath("/api/tags")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var tags OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func (c *OllamaClient) Chat(ctx context.Context, req model.Request) (model.Response, error) {
	payload := c.toPayload(req, false)
	raw, err := json.Marshal(payload)
	if err != nil {
		return model.Response{}, err
	}

	u := c.baseURL.JoinPath("/api/chat")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return model.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return model.Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return model.Response{}, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var out OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return model.Response{}, err
	}

	return model.Response{
		Model:            out.Model,
		Message:          model.Message(out.Message),
		DoneReason:       out.DoneReason,
		PromptTokens:     out.PromptEvalCount,
		CompletionTokens: out.EvalCount,
	}, nil
}

func (c *OllamaClient) StreamChat(ctx context.Context, req model.Request) (io.ReadCloser, error) {
	payload := c.toPayload(req, true)
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	u := c.baseURL.JoinPath("/api/chat")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer func() { _ = resp.Body.Close() }()
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *OllamaClient) ParseStreamChunk(line []byte) (model.StreamChunk, error) {
	var chunk OllamaChatResponse
	if err := json.Unmarshal(line, &chunk); err != nil {
		return model.StreamChunk{}, err
	}

	return model.StreamChunk{
		Model:            chunk.Model,
		Message:          model.Message(chunk.Message),
		Done:             chunk.Done,
		DoneReason:       chunk.DoneReason,
		PromptTokens:     chunk.PromptEvalCount,
		CompletionTokens: chunk.EvalCount,
	}, nil
}

func (c *OllamaClient) toPayload(req model.Request, forceStream bool) map[string]any {
	messages := make([]map[string]string, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, map[string]string{"role": m.Role, "content": m.Content})
	}
	stream := req.Stream
	if forceStream {
		stream = true
	}
	return map[string]any{
		"model":    req.Model,
		"messages": messages,
		"stream":   stream,
	}
}
