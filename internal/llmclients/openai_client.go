package llmclients

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// OpenAIClient adapts the OpenAI HTTP API to the shared llm client contract.
type OpenAIClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	apiKey     string
}

func NewOpenAIClient(baseURL, apiKey string, httpClient *http.Client) (*OpenAIClient, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse openai base url: %w", err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OpenAIClient{baseURL: u, httpClient: httpClient, apiKey: apiKey}, nil
}

func (c *OpenAIClient) Ready(ctx context.Context) error {
	_, err := c.GetModels(ctx)
	return err
}

func (c *OpenAIClient) GetModels(ctx context.Context) (any, error) {
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

	var out OpenAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *OpenAIClient) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	payload := OpenAIChatCompletionRequest{Model: req.Model, Stream: false}
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, OpenAIChatMessage{Role: m.Role, Content: m.Content})
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return ChatResponse{}, err
	}

	u := c.baseURL.JoinPath("/v1/chat/completions")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(raw))
	if err != nil {
		return ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.applyAuth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ChatResponse{}, fmt.Errorf("openai returned status %d", resp.StatusCode)
	}

	var out OpenAIChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatResponse{}, err
	}

	msg := Message{}
	if len(out.Choices) > 0 {
		msg = Message{Role: out.Choices[0].Message.Role, Content: out.Choices[0].Message.Content}
	}

	return ChatResponse{
		Model:            out.Model,
		Message:          msg,
		DoneReason:       "stop",
		PromptTokens:     out.Usage.PromptTokens,
		CompletionTokens: out.Usage.CompletionTokens,
	}, nil
}

func (c *OpenAIClient) StreamChat(ctx context.Context, req ChatRequest) (io.ReadCloser, error) {
	payload := OpenAIChatCompletionRequest{Model: req.Model, Stream: true}
	for _, m := range req.Messages {
		payload.Messages = append(payload.Messages, OpenAIChatMessage{Role: m.Role, Content: m.Content})
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
	c.applyAuth(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("openai returned status %d", resp.StatusCode)
	}

	pipeR, pipeW := io.Pipe()
	go func() {
		defer resp.Body.Close()
		defer pipeW.Close()

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				break
			}

			chunk, err := parseOpenAIDataChunk([]byte(data))
			if err != nil {
				_ = pipeW.CloseWithError(err)
				return
			}
			out, err := json.Marshal(chunk)
			if err != nil {
				_ = pipeW.CloseWithError(err)
				return
			}
			if _, err := pipeW.Write(append(out, '\n')); err != nil {
				return
			}
			if chunk.Done {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			_ = pipeW.CloseWithError(err)
		}
	}()

	return pipeR, nil
}

func (c *OpenAIClient) ParseStreamChunk(line []byte) (StreamChunk, error) {
	var chunk StreamChunk
	if err := json.Unmarshal(line, &chunk); err != nil {
		return StreamChunk{}, err
	}
	return chunk, nil
}

type openAIDataChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
}

func parseOpenAIDataChunk(data []byte) (StreamChunk, error) {
	var payload openAIDataChunk
	if err := json.Unmarshal(data, &payload); err != nil {
		return StreamChunk{}, err
	}

	chunk := StreamChunk{Model: payload.Model}
	if payload.Usage != nil {
		chunk.PromptTokens = payload.Usage.PromptTokens
		chunk.CompletionTokens = payload.Usage.CompletionTokens
	}
	if len(payload.Choices) > 0 {
		chunk.Message = Message{Role: payload.Choices[0].Delta.Role, Content: payload.Choices[0].Delta.Content}
		if payload.Choices[0].FinishReason != nil {
			chunk.Done = true
			chunk.DoneReason = *payload.Choices[0].FinishReason
		}
	}
	if len(payload.Choices) == 0 && payload.Usage != nil {
		chunk.Done = true
		chunk.DoneReason = "stop"
	}
	return chunk, nil
}

func (c *OpenAIClient) applyAuth(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}
