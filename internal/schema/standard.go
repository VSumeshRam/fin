package schema

// Message represents a single chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StandardPayload represents the universal decoupled request
type StandardPayload struct {
	Model     string        `json:"model"`
	Messages  []Message     `json:"messages"`
	Stream    bool          `json:"stream,omitempty"`
	MaxTokens int           `json:"max_tokens,omitempty"`
	Tools     []interface{} `json:"tools,omitempty"`
}

// TokenUsage is the universal struct for tracking FinOps metrics
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Model            string // The exact model string returned by the provider
}

// StreamEvent represents a single SSE chunk delta
type StreamEvent struct {
	Content string
}
