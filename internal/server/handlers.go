package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/chat"
	"github.com/miamollie/solas/internal/tokens"
)

const upstreamProviderLabel = "ollama"

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.chat.Ready(r.Context()) != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	modelsPayload, err := s.chat.GetModels(r.Context())
	if err != nil {
		s.logger.Error("list models failed", "error", err, "provider", upstreamProviderLabel)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := chat.EncodeOpenAIModels(w, modelsPayload); err != nil {
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

	chatReq, err := chat.DecodeOpenAIRequest(r.Body)
	if err != nil {
		status, message := chat.BadRequestStatus(err)
		s.metrics.IncRequests(modelLabel, status)
		http.Error(w, message, status)
		return
	}
	modelLabel = chatReq.Model
	tokenBreakdown := tokens.AnalyzeMessages(chatReq.Messages)
	s.metrics.IncInFlight(upstreamProviderLabel)
	defer s.metrics.DecInFlight(upstreamProviderLabel)

	if chatReq.Stream {
		s.handleChatStream(w, r, chatReq, tokenBreakdown)
		return
	}

	out, status, err := s.chat.Run(r.Context(), chatReq)
	if err != nil {
		s.logger.Error("chat failed", "error", err, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(chatReq.Model, status)
		http.Error(w, "upstream error", status)
		return
	}

	s.metrics.AddTokenUsage(chatReq.Model, out.PromptTokens, tokenBreakdown.CurrentUserTokens, tokenBreakdown.AccumulatedTokens, out.CompletionTokens)
	s.metrics.IncRequests(chatReq.Model, http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if err := chat.EncodeOpenAIResponse(w, out); err != nil {
		s.logger.Error("chat response write failed", "error", err, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(chatReq.Model, http.StatusBadGateway)
	}
}

func (s *Server) handleChatStream(
	w http.ResponseWriter,
	r *http.Request,
	req chat.Request,
	tokenBreakdown tokens.Breakdown,
) {
	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}
	w.Header().Set("Content-Type", chat.OpenAIStreamContentType)

	inputTotalTokens, completionTokens, status, err := s.chat.RunStream(r.Context(), req, func(chunk chat.StreamChunk) error {
		if encodeErr := chat.EncodeOpenAIStreamChunk(w, chunk); encodeErr != nil {
			return encodeErr
		}
		flush()
		return nil
	})
	if err != nil {
		s.logger.Error("chat stream failed", "error", err, "provider", upstreamProviderLabel)
		s.metrics.IncRequests(req.Model, status)
		http.Error(w, "upstream error", status)
		return
	}

	s.metrics.AddTokenUsage(req.Model, inputTotalTokens, tokenBreakdown.CurrentUserTokens, tokenBreakdown.AccumulatedTokens, completionTokens)
	s.metrics.IncRequests(req.Model, http.StatusOK)
}
