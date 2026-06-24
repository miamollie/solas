package chat

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
