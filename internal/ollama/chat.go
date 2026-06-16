package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ChatRequest is a request to Ollama /api/chat.
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatMessage is an Ollama chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse is a non-streaming Ollama response.
type ChatResponse struct {
	Model           string      `json:"model"`
	Message         ChatMessage `json:"message"`
	Done            bool        `json:"done"`
	PromptEvalCount int         `json:"prompt_eval_count"`
	EvalCount       int         `json:"eval_count"`
}

// Chat sends a non-streaming chat request to Ollama.
func (c *Client) Chat(ctx context.Context, reqBody ChatRequest) (ChatResponse, error) {
	u := c.baseURL.JoinPath("/api/chat")
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return ChatResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return ChatResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ChatResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return ChatResponse{}, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	var out ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatResponse{}, err
	}
	return out, nil
}
