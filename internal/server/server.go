package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/miamollie/greenops-local-llm/internal/httpx"
	"github.com/miamollie/greenops-local-llm/internal/ollama"
	"github.com/miamollie/greenops-local-llm/internal/openai"
)

// Server provides HTTP handlers for greenopsd.
type Server struct {
	handler http.Handler
	logger  *slog.Logger
	ready   readinessChecker
}

type readinessChecker interface {
	IsReachable(ctx context.Context) error
	ListModels(ctx context.Context) ([]ollama.TagModel, error)
	Chat(ctx context.Context, reqBody ollama.ChatRequest) (ollama.ChatResponse, error)
}

// New creates a server with baseline routes.
func New(logger *slog.Logger, ready readinessChecker) *Server {
	mux := http.NewServeMux()
	s := &Server{logger: logger, ready: ready}

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("GET /v1/models", s.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", s.handleChatCompletions)

	h := httpx.RequestIDMiddleware(httpx.LoggingMiddleware(logger, mux))
	s.handler = h
	return s
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.ready == nil || s.ready.IsReachable(r.Context()) != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if s.ready == nil {
		http.Error(w, "ollama client unavailable", http.StatusBadGateway)
		return
	}
	models, err := s.ready.ListModels(r.Context())
	if err != nil {
		s.logger.Error("list models failed", "error", err)
		status := http.StatusBadGateway
		if errors.Is(err, context.Canceled) {
			status = http.StatusRequestTimeout
		}
		http.Error(w, "upstream error", status)
		return
	}

	resp := openai.ModelsResponse{Object: "list", Data: make([]openai.ModelInfo, 0, len(models))}
	for _, m := range models {
		resp.Data = append(resp.Data, openai.ModelInfo{ID: m.Name, Object: "model", OwnedBy: "ollama"})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if s.ready == nil {
		http.Error(w, "ollama client unavailable", http.StatusBadGateway)
		return
	}

	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Stream {
		http.Error(w, "streaming not supported", http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}

	ollamaReq := ollama.ChatRequest{
		Model:  req.Model,
		Stream: false,
	}
	for _, m := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, ollama.ChatMessage{Role: m.Role, Content: m.Content})
	}

	out, err := s.ready.Chat(r.Context(), ollamaReq)
	if err != nil {
		s.logger.Error("chat completion failed", "error", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	resp := openai.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   out.Model,
		Choices: []openai.ChatCompletionChoice{{
			Index:        0,
			Message:      openai.ChatMessage{Role: out.Message.Role, Content: out.Message.Content},
			FinishReason: "stop",
		}},
		Usage: openai.Usage{
			PromptTokens:     out.PromptEvalCount,
			CompletionTokens: out.EvalCount,
			TotalTokens:      out.PromptEvalCount + out.EvalCount,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
