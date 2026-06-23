package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/llmclients"
)

func (s *Server) handleModels(client llmclients.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			s.metrics.ObserveDuration("all", time.Since(start))
		}()

		if client == nil {
			s.metrics.IncRequests("all", http.StatusBadGateway)
			http.Error(w, "llm client unavailable", http.StatusBadGateway)
			return
		}

		modelsPayload, err := client.GetModels(r.Context())
		if err != nil {
			s.logger.Error("list models failed", "error", err)
			s.metrics.IncRequests("all", http.StatusBadGateway)
			http.Error(w, "upstream error", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		s.metrics.IncRequests("all", http.StatusOK)
		_ = json.NewEncoder(w).Encode(modelsPayload)
	}
}

func (s *Server) handleChat(client llmclients.Client, codec chatCodec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		modelLabel := "unknown"
		defer func() {
			s.metrics.ObserveDuration(modelLabel, time.Since(start))
		}()

		sharedReq, modelName, badReq := codec.ParseRequest(r)
		if badReq != "" {
			s.metrics.IncRequests("unknown", http.StatusBadRequest)
			http.Error(w, badReq, http.StatusBadRequest)
			return
		}
		modelLabel = modelName

		if sharedReq.Stream {
			s.handleChatStream(w, r, client, codec, sharedReq)
			return
		}

		out, status, err := s.runChat(r.Context(), client, sharedReq)
		if err != nil {
			s.logger.Error("chat failed", "codec", codec.Name(), "error", err)
			s.metrics.IncRequests(modelName, status)
			http.Error(w, "upstream error", status)
			return
		}

		s.metrics.AddTokenUsage(modelName, out.PromptTokens, out.CompletionTokens)
		s.metrics.IncRequests(modelName, http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(codec.EncodeResponse(out))
	}
}

func (s *Server) handleChatStream(
	w http.ResponseWriter,
	r *http.Request,
	client llmclients.Client,
	codec chatCodec,
	req llmclients.ChatRequest,
) {
	emitChunk, finalize := codec.PrepareStream(w, req.Model)
	promptTokens, completionTokens, status, err := s.runChatStream(r.Context(), client, req, func(_ []byte, chunk llmclients.StreamChunk) error {
		return emitChunk(chunk)
	})
	if err != nil {
		s.logger.Error("chat stream failed", "codec", codec.Name(), "error", err)
		s.metrics.IncRequests(req.Model, status)
		http.Error(w, "upstream error", status)
		return
	}

	if err := finalize(); err != nil {
		s.logger.Error("chat stream terminal write failed", "error", err)
		s.metrics.IncRequests(req.Model, http.StatusBadGateway)
		return
	}

	s.metrics.AddTokenUsage(req.Model, promptTokens, completionTokens)
	s.metrics.IncRequests(req.Model, http.StatusOK)
}
