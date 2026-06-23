package streaming

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/miamollie/greenops-local-llm/internal/ollama"
	"github.com/miamollie/greenops-local-llm/internal/openai"
)

// ConsumeNDJSON reads newline-delimited JSON from r and invokes onLine for each JSON line.
// It returns when onLine reports done, EOF is reached, context is canceled, or an error occurs.
func ConsumeNDJSON(ctx context.Context, r io.Reader, onLine func(line []byte) (done bool, err error)) error {
	reader := bufio.NewReader(r)
	readBuf := make([]byte, 4096)
	pending := make([]byte, 0, 4096)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		n, readErr := reader.Read(readBuf)
		if n > 0 {
			pending = append(pending, readBuf[:n]...)
			for {
				nl := bytes.IndexByte(pending, '\n')
				if nl < 0 {
					break
				}
				line := bytes.TrimSpace(pending[:nl])
				pending = pending[nl+1:]
				if len(line) == 0 {
					continue
				}
				done, err := onLine(line)
				if err != nil {
					return err
				}
				if done {
					return nil
				}
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				line := bytes.TrimSpace(pending)
				if len(line) == 0 {
					return nil
				}
				_, err := onLine(line)
				return err
			}
			return readErr
		}
	}
}

// ParseOllamaChunk parses one NDJSON line into an Ollama chat response chunk.
func ParseOllamaChunk(line []byte) (ollama.ChatResponse, error) {
	var chunk ollama.ChatResponse
	if err := json.Unmarshal(line, &chunk); err != nil {
		return ollama.ChatResponse{}, err
	}
	return chunk, nil
}

// ChunkMeta carries terminal state extracted from a streamed chunk.
type ChunkMeta struct {
	Done             bool
	PromptTokens     int
	CompletionTokens int
}

// OpenAIChunkEncoder converts Ollama stream chunks into OpenAI-compatible chunk JSON payloads.
type OpenAIChunkEncoder struct {
	streamID   string
	created    int64
	model      string
	firstChunk bool
}

// NewOpenAIChunkEncoder creates a chunk encoder for one chat-completion stream.
func NewOpenAIChunkEncoder(model string, now time.Time) *OpenAIChunkEncoder {
	return &OpenAIChunkEncoder{
		streamID:   fmt.Sprintf("chatcmpl-%d", now.UnixNano()),
		created:    now.Unix(),
		model:      model,
		firstChunk: true,
	}
}

// Encode marshals a single OpenAI chat.completion.chunk payload for the provided Ollama chunk.
func (e *OpenAIChunkEncoder) Encode(chunk ollama.ChatResponse) ([]byte, ChunkMeta, error) {
	respModel := chunk.Model
	if respModel == "" {
		respModel = e.model
	}

	delta := openai.ChatMessageDelta{Content: chunk.Message.Content}
	if e.firstChunk {
		delta.Role = "assistant"
		if chunk.Message.Role != "" {
			delta.Role = chunk.Message.Role
		}
	}

	var finishReason *string
	meta := ChunkMeta{Done: chunk.Done}
	if chunk.Done {
		reason := "stop"
		finishReason = &reason
		meta.PromptTokens = chunk.PromptEvalCount
		meta.CompletionTokens = chunk.EvalCount
	}

	payload := openai.ChatCompletionChunkResponse{
		ID:      e.streamID,
		Object:  "chat.completion.chunk",
		Created: e.created,
		Model:   respModel,
		Choices: []openai.ChatCompletionChunkChoice{{
			Index:        0,
			Delta:        delta,
			FinishReason: finishReason,
		}},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, ChunkMeta{}, err
	}

	e.firstChunk = false
	return raw, meta, nil
}
