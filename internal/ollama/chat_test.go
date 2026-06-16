package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Stream {
			t.Fatalf("expected stream=false")
		}
		_ = json.NewEncoder(w).Encode(ChatResponse{
			Model:           req.Model,
			Message:         ChatMessage{Role: "assistant", Content: "hello"},
			Done:            true,
			PromptEvalCount: 3,
			EvalCount:       5,
		})
	}))
	defer ts.Close()

	c, err := NewClient(ts.URL, ts.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	resp, err := c.Chat(context.Background(), ChatRequest{
		Model:    "qwen3:32b",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		Stream:   false,
	})
	if err != nil {
		t.Fatalf("chat error: %v", err)
	}
	if resp.Message.Content != "hello" || resp.PromptEvalCount != 3 || resp.EvalCount != 5 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
