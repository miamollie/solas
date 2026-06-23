package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/llmclients"
	"github.com/miamollie/solas/internal/streaming"
)

func (s *Server) runChat(ctx context.Context, client llmclients.Client, req llmclients.ChatRequest) (llmclients.ChatResponse, int, error) {
	if client == nil {
		return llmclients.ChatResponse{}, http.StatusBadGateway, errors.New("llm client unavailable")
	}

	out, err := client.Chat(ctx, req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return llmclients.ChatResponse{}, http.StatusRequestTimeout, err
		}
		return llmclients.ChatResponse{}, http.StatusBadGateway, err
	}
	return out, http.StatusOK, nil
}

func (s *Server) runChatStream(
	ctx context.Context,
	client llmclients.Client,
	req llmclients.ChatRequest,
	onChunk func(rawLine []byte, chunk llmclients.StreamChunk) error,
) (int, int, int, error) {
	if client == nil {
		return 0, 0, http.StatusBadGateway, errors.New("llm client unavailable")
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
		if chunkErr := onChunk(line, chunk); chunkErr != nil {
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
