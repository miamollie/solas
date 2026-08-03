package chat

import (
	"context"
	"io"
)

// Message is an internal chat message.
type Message struct {
	Role    string
	Content string
}

// Request is an internal chat request.
type Request struct {
	Model    string
	Messages []Message
	Stream   bool
}

// Response is an internal non-streaming chat response.
type Response struct {
	Model            string
	Message          Message
	DoneReason       string
	PromptTokens     int
	CompletionTokens int
}

// StreamChunk is an internal streaming response chunk.
type StreamChunk struct {
	Model            string
	Message          Message
	Done             bool
	DoneReason       string
	PromptTokens     int
	CompletionTokens int
}

// Client is the contract for an LLM provider.
type Client interface {
	Ready(ctx context.Context) error
	GetModels(ctx context.Context) (any, error)
	GetVersion(ctx context.Context) (any, error)
	GetRunningModels(ctx context.Context) (any, error)
	Chat(ctx context.Context, req Request) (Response, error)
	StreamChat(ctx context.Context, req Request) (io.ReadCloser, error)
	ParseStreamChunk(line []byte) (StreamChunk, error)
}
