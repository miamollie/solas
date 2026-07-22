package chat

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/streaming"
)

// Service runs chat requests against a Client.
type Service struct {
	client Client
}

// NewService constructs a chat service with a single upstream client.
func NewService(client Client) *Service {
	return &Service{client: client}
}

func (s *Service) configuredClient() (Client, error) {
	if s == nil {
		return nil, errors.New("chat service unavailable")
	}
	if s.client == nil {
		return nil, errors.New("llm client unavailable")
	}
	return s.client, nil
}

// Ready delegates to the underlying client's health check.
func (s *Service) Ready(ctx context.Context) error {
	client, err := s.configuredClient()
	if err != nil {
		return err
	}
	return client.Ready(ctx)
}

// GetModels delegates to the underlying client.
func (s *Service) GetModels(ctx context.Context) (any, error) {
	client, err := s.configuredClient()
	if err != nil {
		return nil, err
	}
	return client.GetModels(ctx)
}

// GetVersion delegates to the underlying provider client version endpoint.
func (s *Service) GetVersion(ctx context.Context) (any, error) {
	client, err := s.configuredClient()
	if err != nil {
		return nil, err
	}
	return client.GetVersion(ctx)
}

// GetRunningModels delegates to the underlying provider running-models endpoint.
func (s *Service) GetRunningModels(ctx context.Context) (any, error) {
	client, err := s.configuredClient()
	if err != nil {
		return nil, err
	}
	return client.GetRunningModels(ctx)
}

// Run executes a non-streaming chat request and returns the response with an HTTP status code.
func (s *Service) Run(ctx context.Context, req Request) (Response, int, error) {
	client, err := s.configuredClient()
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
	req Request,
	onChunk func(StreamChunk) error,
) (int, int, int, error) {
	client, err := s.configuredClient()
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
