package streaming

import (
	"context"
	"strings"
	"testing"
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
