package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/chat"
	"github.com/miamollie/solas/internal/tokens"
)

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReady(provider chat.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.chat.Ready(r.Context(), provider) != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	}
}

func (s *Server) handleModels(provider chat.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modelsPayload, err := s.chat.GetModels(r.Context(), provider)
		if err != nil {
			s.logger.Error("list models failed", "error", err, "provider", provider)
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(modelsPayload)
	}
}

func (s *Server) handleChat(provider chat.Provider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		modelLabel := "unknown"
		defer func() {
			s.metrics.ObserveDuration(modelLabel, time.Since(start))
		}()

		chatReq, err := chat.DecodeRequest(provider, r.Body)
		if err != nil {
			status, message := chat.BadRequestStatus(err)
			s.metrics.IncRequests("unknown", status)
			http.Error(w, message, status)
			return
		}
		modelLabel = chatReq.Model
		tokenBreakdown := tokens.AnalyzeMessages(chatReq.Messages)
		s.metrics.IncInFlight(string(provider))
		defer s.metrics.DecInFlight(string(provider))

		if chatReq.Stream {
			s.handleChatStream(provider, w, r, chatReq, tokenBreakdown)
			return
		}

		out, status, err := s.chat.Run(r.Context(), provider, chatReq)
		if err != nil {
			s.logger.Error("chat failed", "error", err, "provider", provider)
			s.metrics.IncRequests(chatReq.Model, status)
			http.Error(w, "upstream error", status)
			return
		}

		s.metrics.AddTokenUsage(chatReq.Model, out.PromptTokens, tokenBreakdown.CurrentUserTokens, tokenBreakdown.AccumulatedTokens, out.CompletionTokens)
		s.metrics.IncRequests(chatReq.Model, http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		if err := chat.EncodeResponse(provider, w, out); err != nil {
			s.logger.Error("chat response write failed", "error", err, "provider", provider)
			s.metrics.IncRequests(chatReq.Model, http.StatusBadGateway)
		}
	}
}

func (s *Server) handleChatStream(
	provider chat.Provider,
	w http.ResponseWriter,
	r *http.Request,
	req chat.Request,
	tokenBreakdown tokens.Breakdown,
) {
	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}
	w.Header().Set("Content-Type", chat.StreamContentType(provider))

	inputTotalTokens, completionTokens, status, err := s.chat.RunStream(r.Context(), provider, req, func(chunk chat.StreamChunk) error {
		if encodeErr := chat.EncodeStreamChunk(provider, w, chunk); encodeErr != nil {
			return encodeErr
		}
		flush()
		return nil
	})
	if err != nil {
		s.logger.Error("chat stream failed", "error", err, "provider", provider)
		s.metrics.IncRequests(req.Model, status)
		http.Error(w, "upstream error", status)
		return
	}

	s.metrics.AddTokenUsage(req.Model, inputTotalTokens, tokenBreakdown.CurrentUserTokens, tokenBreakdown.AccumulatedTokens, completionTokens)
	s.metrics.IncRequests(req.Model, http.StatusOK)
}
