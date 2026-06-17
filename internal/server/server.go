package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/miamollie/greenops-local-llm/internal/httpx"
	"github.com/miamollie/greenops-local-llm/internal/metrics"
	"github.com/miamollie/greenops-local-llm/internal/ollama"
	"github.com/miamollie/greenops-local-llm/internal/openai"
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
	client, userAgent, remoteIP := requestAttribution(r)

	if s.ready == nil {
		s.metrics.IncRequests("all", http.StatusBadGateway)
		s.metrics.IncClientRequest("all", http.StatusBadGateway, client, userAgent, remoteIP)
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
		s.metrics.IncClientRequest("all", status, client, userAgent, remoteIP)
		http.Error(w, "upstream error", status)
		return
	}

	resp := openai.ModelsResponse{Object: "list", Data: make([]openai.ModelInfo, 0, len(models))}
	for _, m := range models {
		resp.Data = append(resp.Data, openai.ModelInfo{ID: m.Name, Object: "model", OwnedBy: "ollama"})
	}
	s.metrics.IncRequests("all", http.StatusOK)
	s.metrics.IncClientRequest("all", http.StatusOK, client, userAgent, remoteIP)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	modelLabel := "unknown"
	client, userAgent, remoteIP := requestAttribution(r)
	defer func() {
		s.metrics.ObserveDuration(modelLabel, time.Since(start))
	}()

	if s.ready == nil {
		s.metrics.IncRequests("unknown", http.StatusBadGateway)
		s.metrics.IncClientRequest("unknown", http.StatusBadGateway, client, userAgent, remoteIP)
		http.Error(w, "ollama client unavailable", http.StatusBadGateway)
		return
	}

	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.metrics.IncRequests("unknown", http.StatusBadRequest)
		s.metrics.IncClientRequest("unknown", http.StatusBadRequest, client, userAgent, remoteIP)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		s.metrics.IncRequests("unknown", http.StatusBadRequest)
		s.metrics.IncClientRequest("unknown", http.StatusBadRequest, client, userAgent, remoteIP)
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}
	modelLabel = req.Model
	if req.Stream {
		s.handleChatCompletionsStream(w, r, req, client, userAgent, remoteIP)
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
		s.metrics.IncClientRequest(req.Model, http.StatusBadGateway, client, userAgent, remoteIP)
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

	s.metrics.AddTokenUsage(req.Model, out.PromptEvalCount, out.EvalCount)
	s.metrics.IncRequests(req.Model, http.StatusOK)
	s.metrics.IncClientRequest(req.Model, http.StatusOK, client, userAgent, remoteIP)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleChatCompletionsStream(
	w http.ResponseWriter,
	r *http.Request,
	req openai.ChatCompletionRequest,
	client string,
	userAgent string,
	remoteIP string,
) {
	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}

	ollamaReq := ollama.ChatRequest{Model: req.Model, Stream: true}
	for _, m := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, ollama.ChatMessage{Role: m.Role, Content: m.Content})
	}

	stream, err := s.ready.ChatStream(r.Context(), ollamaReq)
	if err != nil {
		s.logger.Error("chat stream failed", "error", err)
		s.metrics.IncRequests(req.Model, http.StatusBadGateway)
		s.metrics.IncClientRequest(req.Model, http.StatusBadGateway, client, userAgent, remoteIP)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	dec := json.NewDecoder(stream)
	streamID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	created := time.Now().Unix()
	firstChunk := true
	totalPromptTokens := 0
	totalCompletionTokens := 0

	for {
		var chunk ollama.ChatResponse
		if err := dec.Decode(&chunk); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			s.logger.Error("chat stream decode failed", "error", err)
			s.metrics.IncRequests(req.Model, http.StatusBadGateway)
			s.metrics.IncClientRequest(req.Model, http.StatusBadGateway, client, userAgent, remoteIP)
			return
		}

		respModel := chunk.Model
		if respModel == "" {
			respModel = req.Model
		}
		delta := openai.ChatMessageDelta{Content: chunk.Message.Content}
		if firstChunk {
			delta.Role = "assistant"
			if chunk.Message.Role != "" {
				delta.Role = chunk.Message.Role
			}
		}

		var finishReason *string
		if chunk.Done {
			reason := "stop"
			if chunk.DoneReason == "" && chunk.Message.Content == "" {
				reason = "stop"
			}
			finishReason = &reason
			totalPromptTokens = chunk.PromptEvalCount
			totalCompletionTokens = chunk.EvalCount
		}

		payload := openai.ChatCompletionChunkResponse{
			ID:      streamID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   respModel,
			Choices: []openai.ChatCompletionChunkChoice{{
				Index:        0,
				Delta:        delta,
				FinishReason: finishReason,
			}},
		}

		raw, err := json.Marshal(payload)
		if err != nil {
			s.logger.Error("chat stream encode failed", "error", err)
			s.metrics.IncRequests(req.Model, http.StatusInternalServerError)
			s.metrics.IncClientRequest(req.Model, http.StatusInternalServerError, client, userAgent, remoteIP)
			return
		}

		if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
			s.logger.Error("chat stream write failed", "error", err)
			s.metrics.IncRequests(req.Model, http.StatusBadGateway)
			s.metrics.IncClientRequest(req.Model, http.StatusBadGateway, client, userAgent, remoteIP)
			return
		}
		flush()
		firstChunk = false

		if chunk.Done {
			break
		}
	}

	if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
		s.logger.Error("chat stream terminal write failed", "error", err)
		s.metrics.IncRequests(req.Model, http.StatusBadGateway)
		s.metrics.IncClientRequest(req.Model, http.StatusBadGateway, client, userAgent, remoteIP)
		return
	}
	flush()

	s.metrics.AddTokenUsage(req.Model, totalPromptTokens, totalCompletionTokens)
	s.metrics.IncRequests(req.Model, http.StatusOK)
	s.metrics.IncClientRequest(req.Model, http.StatusOK, client, userAgent, remoteIP)
}

func requestAttribution(r *http.Request) (client string, userAgent string, remoteIP string) {
	client = r.Header.Get("X-GreenOps-Client")
	userAgent = r.UserAgent()
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remoteIP = host
	if remoteIP == "" {
		remoteIP = "unknown"
	}
	if userAgent == "" {
		userAgent = "unknown"
	}
	if client == "" {
		client = "unknown"
	}
	return client, userAgent, remoteIP
}
