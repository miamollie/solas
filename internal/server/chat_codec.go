package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/llmclients"
	"github.com/miamollie/solas/internal/streaming"
)

type chatCodec interface {
	Name() string
	ParseRequest(r *http.Request) (llmclients.ChatRequest, string, string)
	EncodeResponse(out llmclients.ChatResponse) any
	PrepareStream(w http.ResponseWriter, model string) (emit func(chunk llmclients.StreamChunk) error, finalize func() error)
}

type openAICodec struct{}

func (openAICodec) Name() string {
	return "openai"
}

func (openAICodec) ParseRequest(r *http.Request) (llmclients.ChatRequest, string, string) {
	var req llmclients.OpenAIChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return llmclients.ChatRequest{}, "", "invalid json"
	}
	if req.Model == "" {
		return llmclients.ChatRequest{}, "", "model is required"
	}
	sharedReq := llmclients.ChatRequest{Model: req.Model, Stream: req.Stream}
	for _, m := range req.Messages {
		sharedReq.Messages = append(sharedReq.Messages, llmclients.Message{Role: m.Role, Content: m.Content})
	}
	return sharedReq, req.Model, ""
}

func (openAICodec) EncodeResponse(out llmclients.ChatResponse) any {
	return llmclients.OpenAIChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   out.Model,
		Choices: []llmclients.OpenAIChatCompletionChoice{{
			Index:        0,
			Message:      llmclients.OpenAIChatMessage{Role: out.Message.Role, Content: out.Message.Content},
			FinishReason: "stop",
		}},
		Usage: llmclients.OpenAIUsage{
			PromptTokens:     out.PromptTokens,
			CompletionTokens: out.CompletionTokens,
			TotalTokens:      out.PromptTokens + out.CompletionTokens,
		},
	}
}

func (openAICodec) PrepareStream(w http.ResponseWriter, model string) (func(llmclients.StreamChunk) error, func() error) {
	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	encoder := streaming.NewOpenAIChunkEncoder(model, time.Now())

	emit := func(chunk llmclients.StreamChunk) error {
		raw, _, err := encoder.Encode(chunk)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			return err
		}
		flush()
		return nil
	}

	finalize := func() error {
		if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
			return err
		}
		flush()
		return nil
	}

	return emit, finalize
}

type ollamaCodec struct{}

func (ollamaCodec) Name() string {
	return "ollama"
}

func (ollamaCodec) ParseRequest(r *http.Request) (llmclients.ChatRequest, string, string) {
	var req llmclients.OllamaChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return llmclients.ChatRequest{}, "", "invalid json"
	}
	if req.Model == "" {
		return llmclients.ChatRequest{}, "", "model is required"
	}
	sharedReq := llmclients.ChatRequest{Model: req.Model, Stream: req.Stream}
	for _, m := range req.Messages {
		sharedReq.Messages = append(sharedReq.Messages, llmclients.Message{Role: m.Role, Content: m.Content})
	}
	return sharedReq, req.Model, ""
}

func (ollamaCodec) EncodeResponse(out llmclients.ChatResponse) any {
	return llmclients.OllamaChatResponse{
		Model: out.Model,
		Message: llmclients.OllamaChatMessage{
			Role:    out.Message.Role,
			Content: out.Message.Content,
		},
		Done:            true,
		DoneReason:      out.DoneReason,
		PromptEvalCount: out.PromptTokens,
		EvalCount:       out.CompletionTokens,
	}
}

func (ollamaCodec) PrepareStream(w http.ResponseWriter, _ string) (func(llmclients.StreamChunk) error, func() error) {
	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}

	w.Header().Set("Content-Type", "application/x-ndjson")

	emit := func(chunk llmclients.StreamChunk) error {
		raw, err := json.Marshal(llmclients.OllamaChatResponse{
			Model:           chunk.Model,
			Message:         llmclients.OllamaChatMessage{Role: chunk.Message.Role, Content: chunk.Message.Content},
			Done:            chunk.Done,
			DoneReason:      chunk.DoneReason,
			PromptEvalCount: chunk.PromptTokens,
			EvalCount:       chunk.CompletionTokens,
		})
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s\n", raw); err != nil {
			return err
		}
		flush()
		return nil
	}

	finalize := func() error { return nil }

	return emit, finalize
}
