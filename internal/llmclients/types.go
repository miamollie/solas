package llmclients

import (
	"context"
	"io"
)

// Message is a provider-neutral chat message.
type Message struct {
	Role    string
	Content string
}

// ChatRequest is a provider-neutral chat request.
type ChatRequest struct {
	Model    string
	Messages []Message
	Stream   bool
}

// ChatResponse is a provider-neutral non-streaming chat response.
type ChatResponse struct {
	Model            string
	Message          Message
	DoneReason       string
	PromptTokens     int
	CompletionTokens int
	//TODO add userMessageTokens to highlight split between new messages and context
}

// StreamChunk is a provider-neutral streaming chunk.
type StreamChunk struct {
	Model            string
	Message          Message
	Done             bool
	DoneReason       string
	PromptTokens     int
	CompletionTokens int
}

// Client is the shared contract for LLM provider clients.
type Client interface {
	Ready(ctx context.Context) error
	GetModels(ctx context.Context) (any, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	StreamChat(ctx context.Context, req ChatRequest) (io.ReadCloser, error)
	ParseStreamChunk(line []byte) (StreamChunk, error)
}
