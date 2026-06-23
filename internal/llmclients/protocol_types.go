package llmclients

// OpenAIModelsResponse is OpenAI-compatible response for GET /v1/models.
type OpenAIModelsResponse struct {
	Object string            `json:"object"`
	Data   []OpenAIModelInfo `json:"data"`
}

// OpenAIModelInfo represents an OpenAI-compatible model entry.
type OpenAIModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIChatCompletionRequest is OpenAI-compatible chat completion input.
type OpenAIChatCompletionRequest struct {
	Model    string              `json:"model"`
	Messages []OpenAIChatMessage `json:"messages"`
	Stream   bool                `json:"stream,omitempty"`
}

// OpenAIChatMessage is an OpenAI-compatible chat message.
type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatCompletionResponse is OpenAI-compatible chat completion output.
type OpenAIChatCompletionResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []OpenAIChatCompletionChoice `json:"choices"`
	Usage   OpenAIUsage                  `json:"usage"`
}

// OpenAIChatCompletionChoice is one response choice.
type OpenAIChatCompletionChoice struct {
	Index        int               `json:"index"`
	Message      OpenAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

// OpenAIChatCompletionChunkResponse is an OpenAI-compatible streaming chunk payload.
type OpenAIChatCompletionChunkResponse struct {
	ID      string                            `json:"id"`
	Object  string                            `json:"object"`
	Created int64                             `json:"created"`
	Model   string                            `json:"model"`
	Choices []OpenAIChatCompletionChunkChoice `json:"choices"`
}

// OpenAIChatCompletionChunkChoice is a single OpenAI-compatible stream delta choice.
type OpenAIChatCompletionChunkChoice struct {
	Index        int                    `json:"index"`
	Delta        OpenAIChatMessageDelta `json:"delta"`
	FinishReason *string                `json:"finish_reason"`
}

// OpenAIChatMessageDelta contains incremental streamed token content.
type OpenAIChatMessageDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// OpenAIUsage represents token usage counters.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OllamaTagsResponse is Ollama /api/tags response.
type OllamaTagsResponse struct {
	Models []OllamaTagModel `json:"models"`
}

// OllamaTagModel is a model listed by Ollama.
type OllamaTagModel struct {
	Name string `json:"name"`
}

// OllamaChatRequest is a request to Ollama /api/chat.
type OllamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

// OllamaChatMessage is an Ollama chat message.
type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaChatResponse is a non-streaming/streaming Ollama response chunk.
type OllamaChatResponse struct {
	Model           string            `json:"model"`
	Message         OllamaChatMessage `json:"message"`
	Done            bool              `json:"done"`
	DoneReason      string            `json:"done_reason,omitempty"`
	PromptEvalCount int               `json:"prompt_eval_count"`
	EvalCount       int               `json:"eval_count"`
}
