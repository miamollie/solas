package openai

// ModelsResponse is OpenAI-compatible response for GET /v1/models.
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// ModelInfo represents an OpenAI-compatible model entry.
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// ChatCompletionRequest is OpenAI-compatible chat completion input.
type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

// ChatMessage is an OpenAI-compatible chat message.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionResponse is OpenAI-compatible chat completion output.
type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
}

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionChunkResponse is an OpenAI-compatible streaming chunk payload.
type ChatCompletionChunkResponse struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
}

// ChatCompletionChunkChoice is a single OpenAI-compatible stream delta choice.
type ChatCompletionChunkChoice struct {
	Index        int               `json:"index"`
	Delta        ChatMessageDelta  `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

// ChatMessageDelta contains incremental streamed token content.
type ChatMessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// Usage represents token usage counters.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
