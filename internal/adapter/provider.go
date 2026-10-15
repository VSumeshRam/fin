package adapter

import (
	"net/http"

	"github.com/finops-gateway/internal/schema"
)

// Provider represents a translation layer for a specific LLM API (OpenAI, Anthropic, Gemini)
type Provider interface {
	// FormatRequest translates a StandardPayload struct into the provider's specific wire format bytes
	FormatRequest(payload *schema.StandardPayload) ([]byte, error)

	// FormatResponse converts a provider's raw non-streaming JSON response into standard OpenAI response bytes
	FormatResponse(responseBody []byte) ([]byte, error)

	// ParseUsage extracts TokenUsage from a full JSON response block (non-streaming)
	ParseUsage(responseBody []byte) (*schema.TokenUsage, error)

	// TargetPath returns the endpoint path for this provider (e.g. "/v1/chat/completions" vs "/v1/messages")
	TargetPath() string

	// PrepareRequest modifies the HTTP request (headers, path) for this provider
	PrepareRequest(req *http.Request)
}

// StreamTranslator translates SSE chunks on the fly
type StreamTranslator interface {
	// TranslateChunk translates a raw SSE byte chunk into a universal delta format
	// and returns the target format chunk to flush to the client
	TranslateChunk(chunk []byte) (*schema.StreamEvent, []byte, error)
}