package ollama

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/miamollie/solas/internal/model"
)

var errInvalidJSON = errors.New("invalid json")
var errModelRequired = errors.New("model is required")

// DecodeRequest decodes an Ollama API payload into an internal Request.
func DecodeRequest(r io.Reader) (model.Request, error) {
	var req OllamaChatRequest
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

// EncodeResponse encodes an internal response in Ollama's /api/chat format.
func EncodeResponse(w io.Writer, out model.Response) error {
	return json.NewEncoder(w).Encode(OllamaChatResponse{
		Model:           out.Model,
		Message:         OllamaChatMessage{Role: out.Message.Role, Content: out.Message.Content},
		Done:            true,
		DoneReason:      out.DoneReason,
		PromptEvalCount: out.PromptTokens,
		EvalCount:       out.CompletionTokens,
	})
}

// EncodeStreamChunk writes one NDJSON line in Ollama's stream format.
func EncodeStreamChunk(w io.Writer, chunk model.StreamChunk) error {
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
