package openai

// ModelsResponse is /v1/models compatible payload.
type ModelsResponse struct {
	Object  string      `json:"object"`
	Data    []ModelData `json:"data"`
	OwnedBy string      `json:"owned_by"`
}

// ModelData represents one model entry.
type ModelData struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

// ChatCompletionsRequest is /v1/chat/completions request payload.
type ChatCompletionsRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// ChatMessage is a chat message in OpenAI format.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionsResponse is non-streaming /v1/chat/completions response.
type ChatCompletionsResponse struct {
	ID      string                   `json:"id"`
	Object  string                   `json:"object"`
	Created int64                    `json:"created"`
	Model   string                   `json:"model"`
	Choices []ChatCompletionsChoice  `json:"choices"`
	Usage   ChatCompletionsUsage     `json:"usage"`
}

// ChatCompletionsChoice is one response choice.
type ChatCompletionsChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionsUsage reports token usage.
type ChatCompletionsUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionChunkResponse is one streaming chunk.
type ChatCompletionChunkResponse struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
}

// ChatCompletionChunkChoice is one streamed choice chunk.
type ChatCompletionChunkChoice struct {
	Index        int              `json:"index"`
	Delta        ChatMessageDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
}

// ChatMessageDelta is the streamed delta object.
type ChatMessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}
