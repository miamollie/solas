package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/miamollie/solas/internal/model"
	"github.com/miamollie/solas/internal/ollama"
)

var errInvalidJSON = errors.New("invalid json")
var errModelRequired = errors.New("model is required")

// DecodeRequest decodes an OpenAI-compatible chat completion request.
func DecodeRequest(r io.Reader) (model.Request, error) {
	var req ChatCompletionsRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return model.Request{}, errInvalidJSON
	}
	if req.Model == "" {
		return model.Request{}, errModelRequired
	}

	out := model.Request{Model: req.Model, Stream: req.Stream}
	for _, m := range req.Messages {
		out.Messages = append(out.Messages, model.Message(m))
	}
	return out, nil
}

// BadRequestStatus maps a decode error to its HTTP status and message.
func BadRequestStatus(err error) (int, string) {
	if errors.Is(err, errInvalidJSON) {
		return http.StatusBadRequest, "invalid json"
	}
	if errors.Is(err, errModelRequired) {
		return http.StatusBadRequest, "model is required"
	}
	return http.StatusBadRequest, "invalid request"
}

// EncodeResponse encodes an internal response in OpenAI chat-completions format.
func EncodeResponse(w io.Writer, out model.Response) error {
	finishReason := out.DoneReason
	if finishReason == "" {
		finishReason = "stop"
	}
	now := time.Now()
	resp := ChatCompletionsResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", now.UnixNano()),
		Object:  "chat.completion",
		Created: now.Unix(),
		Model:   out.Model,
		Choices: []ChatCompletionsChoice{{
			Index:        0,
			Message:      ChatMessage{Role: out.Message.Role, Content: out.Message.Content},
			FinishReason: finishReason,
		}},
		Usage: ChatCompletionsUsage{
			PromptTokens:     out.PromptTokens,
			CompletionTokens: out.CompletionTokens,
			TotalTokens:      out.PromptTokens + out.CompletionTokens,
		},
	}
	return json.NewEncoder(w).Encode(resp)
}

// EncodeStreamChunk writes one SSE data frame for OpenAI-compatible streaming.
func EncodeStreamChunk(w io.Writer, chunk model.StreamChunk) error {
	now := time.Now()
	currentModel := chunk.Model
	if currentModel == "" {
		currentModel = "unknown"
	}

	delta := ChatMessageDelta{Content: chunk.Message.Content}
	if chunk.Message.Role != "" {
		delta.Role = chunk.Message.Role
	}

	var finishReason *string
	if chunk.Done {
		reason := chunk.DoneReason
		if reason == "" {
			reason = "stop"
		}
		finishReason = &reason
	}

	payload := ChatCompletionChunkResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", now.UnixNano()),
		Object:  "chat.completion.chunk",
		Created: now.Unix(),
		Model:   currentModel,
		Choices: []ChatCompletionChunkChoice{{
			Index:        0,
			Delta:        delta,
			FinishReason: finishReason,
		}},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", raw); err != nil {
		return err
	}
	if chunk.Done {
		if _, err := fmt.Fprint(w, "data: [DONE]\n\n"); err != nil {
			return err
		}
	}
	return nil
}

// EncodeModels maps upstream model listings into OpenAI's /v1/models schema.
func EncodeModels(w io.Writer, modelsPayload any) error {
	if openAIModels, ok := modelsPayload.(ModelsResponse); ok {
		return json.NewEncoder(w).Encode(openAIModels)
	}

	tags, ok := modelsPayload.(ollama.OllamaTagsResponse)
	if !ok {
		return errInvalidJSON
	}

	out := ModelsResponse{Object: "list", Data: make([]ModelData, 0, len(tags.Models)), OwnedBy: "ollama"}
	for _, providerModel := range tags.Models {
		out.Data = append(out.Data, ModelData{ID: providerModel.Name, Object: "model"})
	}
	return json.NewEncoder(w).Encode(out)
}

const StreamContentType = "text/event-stream"
