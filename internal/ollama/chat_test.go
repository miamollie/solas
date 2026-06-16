package ollama

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestChatStream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !req.Stream {
			t.Fatalf("expected stream=true")
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, strings.Join([]string{
			`{"model":"qwen3:32b","message":{"role":"assistant","content":"hel"},"done":false}`,
			`{"model":"qwen3:32b","message":{"role":"assistant","content":"lo"},"done":true,"prompt_eval_count":3,"eval_count":5}`,
		}, "\n"))
	}))
	defer ts.Close()

	c, err := NewClient(ts.URL, ts.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	body, err := c.ChatStream(context.Background(), ChatRequest{
		Model:    "qwen3:32b",
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		Stream:   true,
	})
	if err != nil {
		t.Fatalf("chat stream error: %v", err)
	}
	defer body.Close()

	dec := json.NewDecoder(body)
	var first ChatResponse
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("decode first chunk: %v", err)
	}
	if first.Message.Content != "hel" || first.Done {
		t.Fatalf("unexpected first chunk: %+v", first)
	}

	var second ChatResponse
	if err := dec.Decode(&second); err != nil {
		t.Fatalf("decode second chunk: %v", err)
	}
	if second.Message.Content != "lo" || !second.Done || second.PromptEvalCount != 3 || second.EvalCount != 5 {
		t.Fatalf("unexpected second chunk: %+v", second)
	}
}
