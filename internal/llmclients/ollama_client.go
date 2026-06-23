package llmclients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// OllamaClient talks directly to Ollama's HTTP API.
type OllamaClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewOllamaClient(baseURL string, httpClient *http.Client) (*OllamaClient, error) {
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

func (c *OllamaClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	payload := c.toPayload(req, false)
	raw, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}

	u := c.baseURL.JoinPath("/api/chat")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ChatResponse{}, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var out struct {
		Model   string `json:"model"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		DoneReason      string `json:"done_reason,omitempty"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatResponse{}, err
	}

	return ChatResponse{
		Model:            out.Model,
		Message:          Message{Role: out.Message.Role, Content: out.Message.Content},
		DoneReason:       out.DoneReason,
		PromptTokens:     out.PromptEvalCount,
		CompletionTokens: out.EvalCount,
	}, nil
}

func (c *OllamaClient) StreamChat(ctx context.Context, req ChatRequest) (io.ReadCloser, error) {
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

func (c *OllamaClient) ParseStreamChunk(line []byte) (StreamChunk, error) {
	var chunk struct {
		Model   string `json:"model"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Done            bool   `json:"done"`
		DoneReason      string `json:"done_reason,omitempty"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.Unmarshal(line, &chunk); err != nil {
		return StreamChunk{}, err
	}

	return StreamChunk{
		Model:            chunk.Model,
		Message:          Message{Role: chunk.Message.Role, Content: chunk.Message.Content},
		Done:             chunk.Done,
		DoneReason:       chunk.DoneReason,
		PromptTokens:     chunk.PromptEvalCount,
		CompletionTokens: chunk.EvalCount,
	}, nil
}

func (c *OllamaClient) toPayload(req ChatRequest, forceStream bool) map[string]any {
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
