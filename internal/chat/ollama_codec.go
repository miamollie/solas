package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
	default:
		return Request{}, errUnsupportedProvider
	}
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
	default:
		return errUnsupportedProvider
	}
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
	default:
		return errUnsupportedProvider
	}
}

// StreamContentType returns the stream response content type for the provider.
func StreamContentType(provider Provider) string {
	switch provider {
	case ProviderOllama:
		return "application/x-ndjson"
	default:
		return "application/octet-stream"
	}
}
