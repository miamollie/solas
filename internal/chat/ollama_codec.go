package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var errInvalidJSON = errors.New("invalid json")
var errModelRequired = errors.New("model is required")
var errUnsupportedProvider = errors.New("unsupported provider")

// DecodeOllamaRequest decodes an Ollama API payload into an internal Request.
func DecodeOllamaRequest(r io.Reader) (Request, error) {
	var req OllamaChatRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return Request{}, errInvalidJSON
	}
	if req.Model == "" {
		return Request{}, errModelRequired
	}

	out := Request{Model: req.Model, Stream: req.Stream}
	for _, m := range req.Messages {
		out.Messages = append(out.Messages, Message{Role: m.Role, Content: m.Content})
	}
	return out, nil
}

// DecodeRequest decodes a provider-specific request payload into an internal Request.
func DecodeRequest(provider Provider, r io.Reader) (Request, error) {
	switch provider {
	case ProviderOllama:
		return DecodeOllamaRequest(r)
	case ProviderOpenAI:
		return DecodeOpenAIRequest(r)
	default:
		return Request{}, errUnsupportedProvider
	}
}

// DecodeOpenAIRequest decodes an OpenAI-compatible chat completion request.
func DecodeOpenAIRequest(r io.Reader) (Request, error) {
	var req OpenAIChatCompletionsRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return Request{}, errInvalidJSON
	}
	if req.Model == "" {
		return Request{}, errModelRequired
	}

	out := Request{Model: req.Model, Stream: req.Stream}
	for _, m := range req.Messages {
		out.Messages = append(out.Messages, Message{Role: m.Role, Content: m.Content})
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
	if errors.Is(err, errUnsupportedProvider) {
		return http.StatusBadRequest, "unsupported provider"
	}
	return http.StatusBadRequest, "invalid request"
}

// EncodeOllamaResponse encodes an internal response in Ollama's /api/chat format.
func EncodeOllamaResponse(w io.Writer, out Response) error {
	return json.NewEncoder(w).Encode(OllamaChatResponse{
		Model:           out.Model,
		Message:         OllamaChatMessage{Role: out.Message.Role, Content: out.Message.Content},
		Done:            true,
		DoneReason:      out.DoneReason,
		PromptEvalCount: out.PromptTokens,
		EvalCount:       out.CompletionTokens,
	})
}

// EncodeResponse encodes an internal response for the selected provider.
func EncodeResponse(provider Provider, w io.Writer, out Response) error {
	switch provider {
	case ProviderOllama:
		return EncodeOllamaResponse(w, out)
	case ProviderOpenAI:
		return EncodeOpenAIResponse(w, out)
	default:
		return errUnsupportedProvider
	}
}

// EncodeOpenAIResponse encodes an internal response in OpenAI chat-completions format.
func EncodeOpenAIResponse(w io.Writer, out Response) error {
	finishReason := out.DoneReason
	if finishReason == "" {
		finishReason = "stop"
	}
	now := time.Now()
	resp := OpenAIChatCompletionsResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", now.UnixNano()),
		Object:  "chat.completion",
		Created: now.Unix(),
		Model:   out.Model,
		Choices: []OpenAIChatCompletionsChoice{{
			Index:        0,
			Message:      OpenAIChatMessage{Role: out.Message.Role, Content: out.Message.Content},
			FinishReason: finishReason,
		}},
		Usage: OpenAIChatCompletionsUsage{
			PromptTokens:     out.PromptTokens,
			CompletionTokens: out.CompletionTokens,
			TotalTokens:      out.PromptTokens + out.CompletionTokens,
		},
	}
	return json.NewEncoder(w).Encode(resp)
}

// EncodeOllamaStreamChunk writes one NDJSON line in Ollama's stream format.
func EncodeOllamaStreamChunk(w io.Writer, chunk StreamChunk) error {
	raw, err := json.Marshal(OllamaChatResponse{
		Model:           chunk.Model,
		Message:         OllamaChatMessage{Role: chunk.Message.Role, Content: chunk.Message.Content},
		Done:            chunk.Done,
		DoneReason:      chunk.DoneReason,
		PromptEvalCount: chunk.PromptTokens,
		EvalCount:       chunk.CompletionTokens,
	})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", raw)
	return err
}

// EncodeStreamChunk writes one provider-specific stream chunk.
func EncodeStreamChunk(provider Provider, w io.Writer, chunk StreamChunk) error {
	switch provider {
	case ProviderOllama:
		return EncodeOllamaStreamChunk(w, chunk)
	case ProviderOpenAI:
		return EncodeOpenAIStreamChunk(w, chunk)
	default:
		return errUnsupportedProvider
	}
}

// EncodeOpenAIStreamChunk writes one SSE data frame for OpenAI-compatible streaming.
func EncodeOpenAIStreamChunk(w io.Writer, chunk StreamChunk) error {
	now := time.Now()
	model := chunk.Model
	if model == "" {
		model = "unknown"
	}

	delta := OpenAIChatMessageDelta{Content: chunk.Message.Content}
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

	payload := OpenAIChatCompletionChunkResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", now.UnixNano()),
		Object:  "chat.completion.chunk",
		Created: now.Unix(),
		Model:   model,
		Choices: []OpenAIChatCompletionChunkChoice{{
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

// EncodeModels encodes provider-specific models payloads.
func EncodeModels(provider Provider, w io.Writer, modelsPayload any) error {
	switch provider {
	case ProviderOllama:
		return json.NewEncoder(w).Encode(modelsPayload)
	case ProviderOpenAI:
		return EncodeOpenAIModels(w, modelsPayload)
	default:
		return errUnsupportedProvider
	}
}

// EncodeOpenAIModels maps upstream model listings into OpenAI's /v1/models schema.
func EncodeOpenAIModels(w io.Writer, modelsPayload any) error {
	if openAIModels, ok := modelsPayload.(OpenAIModelsResponse); ok {
		return json.NewEncoder(w).Encode(openAIModels)
	}

	tags, ok := modelsPayload.(OllamaTagsResponse)
	if !ok {
		return errInvalidJSON
	}

	out := OpenAIModelsResponse{Object: "list", Data: make([]OpenAIModelData, 0, len(tags.Models))}
	for _, model := range tags.Models {
		out.Data = append(out.Data, OpenAIModelData{ID: model.Name, Object: "model"})
	}
	return json.NewEncoder(w).Encode(out)
}

// StreamContentType returns the stream response content type for the provider.
func StreamContentType(provider Provider) string {
	switch provider {
	case ProviderOllama:
		return "application/x-ndjson"
	case ProviderOpenAI:
		return "text/event-stream"
	default:
		return "application/octet-stream"
	}
}
