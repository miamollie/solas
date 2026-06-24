package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/miamollie/solas/internal/chat"
)

// Client talks directly to OpenAI-compatible HTTP APIs.
type Client struct {
	baseURL    *url.URL
	apiKey     string
	httpClient *http.Client
}

func New(baseURL, apiKey string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse openai base url: %w", err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{baseURL: u, apiKey: apiKey, httpClient: httpClient}, nil
}

func (c *Client) Ready(ctx context.Context) error {
	_, err := c.GetModels(ctx)
	return err
}

func (c *Client) GetModels(ctx context.Context) (any, error) {
	u := c.baseURL.JoinPath("/v1/models")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	c.applyAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openai returned status %d", resp.StatusCode)
	}

	var models chat.OpenAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, err
	}
	return models, nil
}

func (c *Client) Chat(ctx context.Context, req chat.Request) (chat.Response, error) {
	payload := chat.OpenAIChatCompletionsRequest{Model: req.Model, Stream: false}
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, chat.OpenAIChatMessage{Role: m.Role, Content: m.Content})
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return chat.Response{}, err
	}

	u := c.baseURL.JoinPath("/v1/chat/completions")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return chat.Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.applyAuth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return chat.Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return chat.Response{}, fmt.Errorf("openai returned status %d", resp.StatusCode)
	}

	var out chat.OpenAIChatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return chat.Response{}, err
	}
	if len(out.Choices) == 0 {
		return chat.Response{}, errors.New("openai response missing choices")
	}

	return chat.Response{
		Model:            out.Model,
		Message:          chat.Message{Role: out.Choices[0].Message.Role, Content: out.Choices[0].Message.Content},
		DoneReason:       out.Choices[0].FinishReason,
		PromptTokens:     out.Usage.PromptTokens,
		CompletionTokens: out.Usage.CompletionTokens,
	}, nil
}

func (c *Client) StreamChat(ctx context.Context, req chat.Request) (io.ReadCloser, error) {
	payload := chat.OpenAIChatCompletionsRequest{Model: req.Model, Stream: true}
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, chat.OpenAIChatMessage{Role: m.Role, Content: m.Content})
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	u := c.baseURL.JoinPath("/v1/chat/completions")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	c.applyAuth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("openai returned status %d", resp.StatusCode)
	}

	pr, pw := io.Pipe()
	go func() {
		defer resp.Body.Close()
		defer pw.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			payloadLine := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payloadLine == "[DONE]" {
				return
			}
			if _, err := fmt.Fprintf(pw, "%s\n", payloadLine); err != nil {
				_ = pw.CloseWithError(err)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			_ = pw.CloseWithError(err)
		}
	}()

	return pr, nil
}

func (c *Client) ParseStreamChunk(line []byte) (chat.StreamChunk, error) {
	var chunk chat.OpenAIChatCompletionChunkResponse
	if err := json.Unmarshal(line, &chunk); err != nil {
		return chat.StreamChunk{}, err
	}
	if len(chunk.Choices) == 0 {
		return chat.StreamChunk{}, errors.New("openai stream chunk missing choices")
	}

	choice := chunk.Choices[0]
	out := chat.StreamChunk{
		Model:   chunk.Model,
		Message: chat.Message{Role: choice.Delta.Role, Content: choice.Delta.Content},
	}
	if choice.FinishReason != nil {
		out.Done = true
		out.DoneReason = *choice.FinishReason
	}
	return out, nil
}

func (c *Client) applyAuth(req *http.Request) {
	if strings.TrimSpace(c.apiKey) == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
}
