package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miamollie/greenops-local-llm/internal/httpx"
	"github.com/miamollie/greenops-local-llm/internal/metrics"
	"github.com/miamollie/greenops-local-llm/internal/ollama"
	"github.com/miamollie/greenops-local-llm/internal/openai"
	"github.com/miamollie/greenops-local-llm/internal/streaming"
)

// Server provides HTTP handlers for greenopsd.
type Server struct {
	handler http.Handler
	logger  *slog.Logger
	ready   readinessChecker
	metrics *metrics.Metrics
}

type readinessChecker interface {
	IsReachable(ctx context.Context) error
	ListModels(ctx context.Context) ([]ollama.TagModel, error)
	Chat(ctx context.Context, reqBody ollama.ChatRequest) (ollama.ChatResponse, error)
	ChatStream(ctx context.Context, reqBody ollama.ChatRequest) (io.ReadCloser, error)
}

// New creates a server with baseline routes.
func New(logger *slog.Logger, ready readinessChecker, met *metrics.Metrics) *Server {
	if met == nil {
		met = metrics.New()
	}
	s := &Server{logger: logger, ready: ready, metrics: met}

	r := chi.NewRouter()
	r.Use(httpx.RequestIDMiddleware)

	// infra routes — no logging
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)
	r.Handle("/metrics", s.metrics.Handler())

	// openAI group — request logging applied only here
	r.Group(func(r chi.Router) {
		r.Use(httpx.LoggingMiddleware(logger))
		r.Get("/v1/models", s.handleModels)
		r.Post("/v1/chat/completions", s.handleChatCompletions)
	})

	s.handler = r
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
	start := time.Now()
	defer s.metrics.ObserveDuration("all", time.Since(start))

	if s.ready == nil {
		s.metrics.IncRequests("all", http.StatusBadGateway)
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
		s.metrics.IncRequests("all", status)
		http.Error(w, "upstream error", status)
		return
	}

	resp := openai.ModelsResponse{Object: "list", Data: make([]openai.ModelInfo, 0, len(models))}
	for _, m := range models {
		resp.Data = append(resp.Data, openai.ModelInfo{ID: m.Name, Object: "model", OwnedBy: "ollama"})
	}
	s.metrics.IncRequests("all", http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	modelLabel := "unknown"
	defer func() {
		s.metrics.ObserveDuration(modelLabel, time.Since(start))
	}()

	if s.ready == nil {
		s.metrics.IncRequests("unknown", http.StatusBadGateway)
		http.Error(w, "ollama client unavailable", http.StatusBadGateway)
		return
	}

	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.metrics.IncRequests("unknown", http.StatusBadRequest)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		s.metrics.IncRequests("unknown", http.StatusBadRequest)
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}
	modelLabel = req.Model
	if req.Stream {
		s.handleChatCompletionsStream(w, r, req)
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
		s.metrics.IncRequests(req.Model, http.StatusBadGateway)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	//missing role?
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

	s.metrics.AddTokenUsage(req.Model, out.PromptEvalCount, out.EvalCount)
	s.metrics.IncRequests(req.Model, http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatCompletionsStream(
	w http.ResponseWriter,
	r *http.Request,
	req openai.ChatCompletionRequest,
) {
	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}

	ollamaReq := ollama.ChatRequest{Model: req.Model, Stream: true}
	for _, m := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, ollama.ChatMessage{Role: m.Role, Content: m.Content})
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	stream, err := s.ready.ChatStream(ctx, ollamaReq)
	if err != nil {
		s.logger.Error("chat stream failed", "error", err)
		s.metrics.IncRequests(req.Model, http.StatusBadGateway)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	encoder := streaming.NewOpenAIChunkEncoder(req.Model, time.Now())
	totalPromptTokens := 0
	totalCompletionTokens := 0

	emitChunk := func(line []byte) (bool, error) {
		chunk, err := streaming.ParseOllamaChunk(line)
		if err != nil {
			s.logger.Error("chat stream decode failed", "error", err)
			s.metrics.IncRequests(req.Model, http.StatusBadGateway)
			return false, err
		}

		raw, meta, err := encoder.Encode(chunk)
		if err != nil {
			s.logger.Error("chat stream encode failed", "error", err)
			s.metrics.IncRequests(req.Model, http.StatusInternalServerError)
			return false, err
		}

		if meta.Done {
			totalPromptTokens = meta.PromptTokens
			totalCompletionTokens = meta.CompletionTokens
		}

		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			s.logger.Error("chat stream write failed", "error", err)
			s.metrics.IncRequests(req.Model, http.StatusBadGateway)
			return false, err
		}
		flush()

		return meta.Done, nil
	}

	if err := streaming.ConsumeNDJSON(ctx, stream, emitChunk); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			s.logger.Warn("chat stream context canceled", "error", err)
			s.metrics.IncRequests(req.Model, http.StatusRequestTimeout)
			return
		}
		s.logger.Error("chat stream read failed", "error", err)
		s.metrics.IncRequests(req.Model, http.StatusBadGateway)
		return
	}

	if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
		s.logger.Error("chat stream terminal write failed", "error", err)
		s.metrics.IncRequests(req.Model, http.StatusBadGateway)
		return
	}
	flush()

	s.metrics.AddTokenUsage(req.Model, totalPromptTokens, totalCompletionTokens)
	s.metrics.IncRequests(req.Model, http.StatusOK)
}
