package chat

// OpenAIModelsResponse is /v1/models compatible payload.
type OpenAIModelsResponse struct {
	Object string            `json:"object"`
	Data   []OpenAIModelData `json:"data"`
}

// OpenAIModelData represents one model entry.
type OpenAIModelData struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

// OpenAIChatCompletionsRequest is /v1/chat/completions request payload.
type OpenAIChatCompletionsRequest struct {
	Model    string              `json:"model"`
	Messages []OpenAIChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

// OpenAIChatMessage is a chat message in OpenAI format.
type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatCompletionsResponse is non-streaming /v1/chat/completions response.
type OpenAIChatCompletionsResponse struct {
	ID      string                        `json:"id"`
	Object  string                        `json:"object"`
	Created int64                         `json:"created"`
	Model   string                        `json:"model"`
	Choices []OpenAIChatCompletionsChoice `json:"choices"`
	Usage   OpenAIChatCompletionsUsage    `json:"usage"`
}

// OpenAIChatCompletionsChoice is one response choice.
type OpenAIChatCompletionsChoice struct {
	Index        int               `json:"index"`
	Message      OpenAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

// OpenAIChatCompletionsUsage reports token usage.
type OpenAIChatCompletionsUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIChatCompletionChunkResponse is one streaming chunk.
type OpenAIChatCompletionChunkResponse struct {
	ID      string                            `json:"id"`
	Object  string                            `json:"object"`
	Created int64                             `json:"created"`
	Model   string                            `json:"model"`
	Choices []OpenAIChatCompletionChunkChoice `json:"choices"`
}

// OpenAIChatCompletionChunkChoice is one streamed choice chunk.
type OpenAIChatCompletionChunkChoice struct {
	Index        int                    `json:"index"`
	Delta        OpenAIChatMessageDelta `json:"delta"`
	FinishReason *string                `json:"finish_reason"`
}

// OpenAIChatMessageDelta is the streamed delta object.
type OpenAIChatMessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}
