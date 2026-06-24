package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/streaming"
)

// Service runs chat requests against a Client.
type Service struct {
	clients map[Provider]Client
}

// NewService constructs a chat service with provider-backed clients.
func NewService(clients map[Provider]Client) *Service {
	copyClients := make(map[Provider]Client, len(clients))
	for provider, client := range clients {
		if client != nil {
			copyClients[provider] = client
		}
	}
	return &Service{clients: copyClients}
}

func (s *Service) client(provider Provider) (Client, error) {
	if s == nil {
		return nil, errors.New("chat service unavailable")
	}
	client, ok := s.clients[provider]
	if !ok || client == nil {
		return nil, fmt.Errorf("llm client unavailable for provider %q", provider)
	}
	return client, nil
}

// Ready delegates to the underlying client's health check.
func (s *Service) Ready(ctx context.Context, provider Provider) error {
	client, err := s.client(provider)
	if err != nil {
		return err
	}
	return client.Ready(ctx)
}

// GetModels delegates to the underlying client.
func (s *Service) GetModels(ctx context.Context, provider Provider) (any, error) {
	client, err := s.client(provider)
	if err != nil {
		return nil, err
	}
	return client.GetModels(ctx)
}

// Run executes a non-streaming chat request and returns the response with an HTTP status code.
func (s *Service) Run(ctx context.Context, provider Provider, req Request) (Response, int, error) {
	client, err := s.client(provider)
	if err != nil {
		return Response{}, http.StatusBadGateway, err
	}
	out, err := client.Chat(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Response{}, http.StatusRequestTimeout, err
		}
		return Response{}, http.StatusBadGateway, err
	}
	return out, http.StatusOK, nil
}

// RunStream executes a streaming chat request, calling onChunk for each chunk.
// Returns (promptTokens, completionTokens, httpStatus, error).
func (s *Service) RunStream(
	ctx context.Context,
	provider Provider,
	req Request,
	onChunk func(StreamChunk) error,
) (int, int, int, error) {
	client, err := s.client(provider)
	if err != nil {
		return 0, 0, http.StatusBadGateway, err
	}

	streamCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	stream, err := client.StreamChat(streamCtx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return 0, 0, http.StatusRequestTimeout, err
		}
		return 0, 0, http.StatusBadGateway, err
	}
	defer stream.Close()

	totalPromptTokens := 0
	totalCompletionTokens := 0

	consumeErr := streaming.ConsumeNDJSON(streamCtx, stream, func(line []byte) (bool, error) {
		chunk, parseErr := client.ParseStreamChunk(line)
		if parseErr != nil {
			return false, parseErr
		}
		if chunk.Done {
			totalPromptTokens = chunk.PromptTokens
			totalCompletionTokens = chunk.CompletionTokens
		}
		if chunkErr := onChunk(chunk); chunkErr != nil {
			return false, chunkErr
		}
		return chunk.Done, nil
	})
	if consumeErr != nil {
		if errors.Is(consumeErr, context.Canceled) || errors.Is(consumeErr, context.DeadlineExceeded) {
			return 0, 0, http.StatusRequestTimeout, consumeErr
		}
		return 0, 0, http.StatusBadGateway, consumeErr
	}

	return totalPromptTokens, totalCompletionTokens, http.StatusOK, nil
}
