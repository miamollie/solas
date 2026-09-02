package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/model"
	"github.com/miamollie/solas/internal/openai"
	"github.com/miamollie/solas/internal/streaming"
	"github.com/miamollie/solas/internal/tokens"
)

const upstreamProviderLabel = "ollama"

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	client, err := s.configuredClient()
	if err != nil || client.Ready(r.Context()) != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	client, err := s.configuredClient()
	if err != nil {
		s.logger.Error("list models failed", "error", err, "provider", upstreamProviderLabel)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	modelsPayload, err := client.GetModels(r.Context())
	if err != nil {
		s.logger.Error("list models failed", "error", err, "provider", upstreamProviderLabel)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := openai.EncodeModels(w, modelsPayload); err != nil {
		s.logger.Error("encode models failed", "error", err, "provider", upstreamProviderLabel)
		http.Error(w, "upstream error", http.StatusBadGateway)
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	modelLabel := "unknown"
	defer func() {
		s.metrics.ObserveDuration(modelLabel, time.Since(start))
	}()

	chatReq, err := openai.DecodeRequest(r.Body)
	if err != nil {
		status, message := openai.BadRequestStatus(err)
		s.metrics.IncRequests(modelLabel, status)
		http.Error(w, message, status)
		return
	}
	modelLabel = chatReq.Model
	tokenBreakdown := tokens.AnalyzeMessages(chatReq.Messages)

	if chatReq.Stream {
		s.handleChatStream(w, r, chatReq, tokenBreakdown)
		return
	}

	client, err := s.configuredClient()
	if err != nil {
		s.logger.Error("chat failed", "error", err, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(chatReq.Model, http.StatusBadGateway)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	out, err := client.Chat(r.Context(), chatReq)
	if err != nil {
		status := mapUpstreamErrorToStatus(err)
		s.logger.Error("chat failed", "error", err, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(chatReq.Model, status)
		http.Error(w, "upstream error", status)
		return
	}

	s.metrics.AddTokenUsage(chatReq.Model, out.PromptTokens, tokenBreakdown.CurrentUserTokens, tokenBreakdown.AccumulatedTokens, out.CompletionTokens)
	w.Header().Set("Content-Type", "application/json")
	if err := openai.EncodeResponse(w, out); err != nil {
		s.logger.Error("chat response write failed", "error", err, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(chatReq.Model, http.StatusBadGateway)
	}
	s.metrics.IncRequests(chatReq.Model, http.StatusOK)
}

func (s *Server) handleChatStream(
	w http.ResponseWriter,
	r *http.Request,
	req model.Request,
	tokenBreakdown tokens.Breakdown,
) {
	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}
	w.Header().Set("Content-Type", openai.StreamContentType)

	client, err := s.configuredClient()
	if err != nil {
		s.logger.Error("chat stream failed", "error", err, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(req.Model, http.StatusBadGateway)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	streamCtx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	stream, err := client.StreamChat(streamCtx, req)
	if err != nil {
		status := mapUpstreamErrorToStatus(err)
		s.logger.Error("chat stream failed", "error", err, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(req.Model, status)
		http.Error(w, "upstream error", status)
		return
	}
	defer func() { _ = stream.Close() }()

	inputTotalTokens := 0
	completionTokens := 0

	consumeErr := streaming.ConsumeNDJSON(streamCtx, stream, func(line []byte) (bool, error) {
		chunk, parseErr := client.ParseStreamChunk(line)
		if parseErr != nil {
			return false, parseErr
		}
		if chunk.Done {
			inputTotalTokens = chunk.PromptTokens
			completionTokens = chunk.CompletionTokens
		}
		if encodeErr := openai.EncodeStreamChunk(w, chunk); encodeErr != nil {
			return false, encodeErr
		}
		flush()
		return chunk.Done, nil
	})
	if consumeErr != nil {
		status := mapUpstreamErrorToStatus(consumeErr)
		s.logger.Error("chat stream failed", "error", consumeErr, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(req.Model, status)
		http.Error(w, "upstream error", status)
		return
	}

	s.metrics.AddTokenUsage(req.Model, inputTotalTokens, tokenBreakdown.CurrentUserTokens, tokenBreakdown.AccumulatedTokens, completionTokens)
	s.metrics.IncRequests(req.Model, http.StatusOK)
}

func (s *Server) configuredClient() (model.Client, error) {
	if s.llmClient == nil {
		return nil, errors.New("llm client unavailable")
	}
	return s.llmClient, nil
}

func mapUpstreamErrorToStatus(err error) int {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return http.StatusRequestTimeout
	}
	return http.StatusBadGateway
}
