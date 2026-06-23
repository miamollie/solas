package streaming

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/miamollie/solas/internal/llmclients"
)

func TestConsumeNDJSONReadsLinesAndStopsOnDone(t *testing.T) {
	input := "{\"n\":1}\n\n{\"n\":2}\n{\"n\":3}\n"
	ctx := context.Background()
	seen := 0

	err := ConsumeNDJSON(ctx, strings.NewReader(input), func(line []byte) (bool, error) {
		seen++
		return seen == 2, nil
	})
	if err != nil {
		t.Fatalf("ConsumeNDJSON returned error: %v", err)
	}
	if seen != 2 {
		t.Fatalf("expected 2 lines to be processed, got %d", seen)
	}
}

func TestConsumeNDJSONConsumesTrailingLineOnEOF(t *testing.T) {
	ctx := context.Background()
	seen := 0

	err := ConsumeNDJSON(ctx, strings.NewReader("{\"n\":1}"), func(line []byte) (bool, error) {
		seen++
		if string(line) != "{\"n\":1}" {
			t.Fatalf("unexpected line %q", string(line))
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("ConsumeNDJSON returned error: %v", err)
	}
	if seen != 1 {
		t.Fatalf("expected 1 line to be processed, got %d", seen)
	}
}

func TestConsumeNDJSONHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ConsumeNDJSON(ctx, strings.NewReader("{\"n\":1}\n"), func(_ []byte) (bool, error) {
		return false, nil
	})
	if err == nil {
		t.Fatalf("expected context cancellation error")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestOpenAIChunkEncoderEncodesFirstAndTerminalChunks(t *testing.T) {
	now := time.Unix(100, 0)
	enc := NewOpenAIChunkEncoder("qwen3:32b", now)

	firstRaw, firstMeta, err := enc.Encode(llmclients.StreamChunk{
		Model: "",
		Message: llmclients.Message{
			Role:    "assistant",
			Content: "hel",
		},
		Done: false,
	})
	if err != nil {
		t.Fatalf("encode first chunk: %v", err)
	}
	if firstMeta.Done {
		t.Fatalf("expected first chunk meta done=false")
	}

	var firstPayload map[string]any
	if err := json.Unmarshal(firstRaw, &firstPayload); err != nil {
		t.Fatalf("decode first payload: %v", err)
	}
	if firstPayload["object"] != "chat.completion.chunk" {
		t.Fatalf("unexpected object: %v", firstPayload["object"])
	}

	secondRaw, secondMeta, err := enc.Encode(llmclients.StreamChunk{
		Model: "qwen3:32b",
		Message: llmclients.Message{
			Role:    "assistant",
			Content: "lo",
		},
		Done:             true,
		PromptTokens:     3,
		CompletionTokens: 5,
	})
	if err != nil {
		t.Fatalf("encode terminal chunk: %v", err)
	}
	if !secondMeta.Done || secondMeta.PromptTokens != 3 || secondMeta.CompletionTokens != 5 {
		t.Fatalf("unexpected terminal meta: %+v", secondMeta)
	}

	terminalText, err := io.ReadAll(strings.NewReader(string(secondRaw)))
	if err != nil {
		t.Fatalf("read terminal payload: %v", err)
	}
	if !strings.Contains(string(terminalText), "\"finish_reason\":\"stop\"") {
		t.Fatalf("expected finish_reason stop in terminal payload: %s", string(terminalText))
	}
}
