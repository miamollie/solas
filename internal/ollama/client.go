package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/miamollie/solas/internal/chat"
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
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var tags OllamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func (c *OllamaClient) Chat(ctx context.Context, req chat.Request) (chat.Response, error) {
	payload := c.toPayload(req, false)
	raw, err := json.Marshal(payload)
	if err != nil {
		return chat.Response{}, err
	}

	u := c.baseURL.JoinPath("/api/chat")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return chat.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return chat.Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return chat.Response{}, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var out OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return chat.Response{}, err
	}

	return chat.Response{
		Model:            out.Model,
		Message:          chat.Message{Role: out.Message.Role, Content: out.Message.Content},
		DoneReason:       out.DoneReason,
		PromptTokens:     out.PromptEvalCount,
		CompletionTokens: out.EvalCount,
	}, nil
}

func (c *OllamaClient) StreamChat(ctx context.Context, req chat.Request) (io.ReadCloser, error) {
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
		defer resp.Body.Close()
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (c *OllamaClient) ParseStreamChunk(line []byte) (chat.StreamChunk, error) {
	var chunk OllamaChatResponse
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

func (c *OllamaClient) toPayload(req chat.Request, forceStream bool) map[string]any {
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
