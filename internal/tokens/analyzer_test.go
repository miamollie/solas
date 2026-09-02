package tokens

import (
	"testing"

	"github.com/miamollie/solas/internal/model"
)

func TestAnalyzeMessagesMostRecentUserAndAccumulated(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "policy ctx"},
		{Role: "user", Content: "older question here"},
		{Role: "assistant", Content: "older answer"},
		{Role: "user", Content: "latest question"},
	}

	b := AnalyzeMessages(messages)

	if b.CurrentUserTokens != 2 {
		t.Fatalf("expected current user tokens=2, got %d", b.CurrentUserTokens)
	}
	if b.AccumulatedTokens != 7 {
		t.Fatalf("expected accumulated tokens=7, got %d", b.AccumulatedTokens)
	}
}

func TestAnalyzeMessagesWithoutUserMessage(t *testing.T) {
	messages := []model.Message{
		{Role: "system", Content: "one two"},
		{Role: "assistant", Content: "three"},
	}

	b := AnalyzeMessages(messages)

	if b.CurrentUserTokens != 0 {
		t.Fatalf("expected current user tokens=0, got %d", b.CurrentUserTokens)
	}
	if b.AccumulatedTokens != 3 {
		t.Fatalf("expected accumulated tokens=3, got %d", b.AccumulatedTokens)
	}
}
