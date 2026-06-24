package tokens

import (
	"strings"

	"github.com/miamollie/solas/internal/chat"
)

// Breakdown captures estimated input token splits from the request payload.
type Breakdown struct {
	CurrentUserTokens int
	AccumulatedTokens int
}

// AnalyzeMessages estimates token splits by treating fields-separated words as token units.
// CurrentUserTokens is the most recent user message; AccumulatedTokens is everything else.
func AnalyzeMessages(messages []chat.Message) Breakdown {
	total := 0
	lastUserIdx := -1
	lastUserTokens := 0

	for i, m := range messages {
		tok := estimateContentTokens(m.Content)
		total += tok
		if strings.EqualFold(strings.TrimSpace(m.Role), "user") {
			lastUserIdx = i
			lastUserTokens = tok
		}
	}

	if lastUserIdx == -1 {
		return Breakdown{CurrentUserTokens: 0, AccumulatedTokens: total}
	}

	return Breakdown{
		CurrentUserTokens: lastUserTokens,
		AccumulatedTokens: total - lastUserTokens,
	}
}

func estimateContentTokens(content string) int {
	return len(strings.Fields(content))
}
