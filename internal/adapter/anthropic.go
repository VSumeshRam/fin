package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/finops-gateway/internal/schema"
)

type AnthropicAdapter struct{}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Role       string                  `json:"role"`
	Model      string                  `json:"model"`
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (a *AnthropicAdapter) FormatRequest(payload *schema.StandardPayload) ([]byte, error) {
	req := anthropicRequest{
		Model:     payload.Model,
		MaxTokens: 4096, // Anthropic requires max_tokens
		Stream:    payload.Stream,
	}

	if req.Model == "" || !strings.Contains(req.Model, "claude") {
		req.Model = "claude-3-5-sonnet-20241022"
	}

	var systemParts []string
	for _, msg := range payload.Messages {
		if strings.ToLower(msg.Role) == "system" {
			systemParts = append(systemParts, msg.Content)
		} else {
			role := msg.Role
			if role != "user" && role != "assistant" {
				role = "user"
			}
			req.Messages = append(req.Messages, anthropicMessage{
				Role:    role,
				Content: msg.Content,
			})
		}
	}

	if len(systemParts) > 0 {
		req.System = strings.Join(systemParts, "\n\n")
	}

	return json.Marshal(req)
}

func (a *AnthropicAdapter) FormatResponse(responseBody []byte) ([]byte, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return nil, err
	}

	// Extract full response text
	var textBuilder strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			textBuilder.WriteString(block.Text)
		}
	}

	finishReason := "stop"
	if resp.StopReason == "max_tokens" {
		finishReason = "length"
	}

	// Map to standard OpenAI chat completion schema
	openAIResp := map[string]interface{}{
		"id":      "chatcmpl-" + resp.ID,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   resp.Model,
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": textBuilder.String(),
				},
				"finish_reason": finishReason,
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     resp.Usage.InputTokens,
			"completion_tokens": resp.Usage.OutputTokens,
			"total_tokens":      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	return json.Marshal(openAIResp)
}

func (a *AnthropicAdapter) ParseUsage(responseBody []byte) (*schema.TokenUsage, error) {
	var resp anthropicResponse
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		return nil, err
	}

	return &schema.TokenUsage{
		Model:            resp.Model,
		PromptTokens:     resp.Usage.InputTokens,
		CompletionTokens: resp.Usage.OutputTokens,
		TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
	}, nil
}

func (a *AnthropicAdapter) TargetPath() string {
	return "/v1/messages"
}

func (a *AnthropicAdapter) PrepareRequest(req *http.Request) {
	req.URL.Path = "/v1/messages"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if auth := req.Header.Get("Authorization"); auth != "" {
		apiKey := strings.TrimPrefix(auth, "Bearer ")
		req.Header.Set("x-api-key", strings.TrimSpace(apiKey))
	}
}

// Anthropic SSE structs
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
}

func (a *AnthropicAdapter) TranslateChunk(chunk []byte) (*schema.StreamEvent, []byte, error) {
	lines := bytes.Split(chunk, []byte("\n"))
	var dataLine []byte
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("data:")) {
			dataLine = bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
			break
		}
	}

	if dataLine == nil {
		return nil, nil, nil
	}

	if bytes.Equal(dataLine, []byte("[DONE]")) {
		return &schema.StreamEvent{Content: "[DONE]"}, []byte("data: [DONE]\n\n"), nil
	}

	var event anthropicStreamEvent
	if err := json.Unmarshal(dataLine, &event); err != nil {
		return nil, nil, err
	}

	if event.Type == "content_block_delta" && event.Delta.Text != "" {
		text := event.Delta.Text
		// Format into OpenAI SSE delta chunk
		escapedText, _ := json.Marshal(text)
		openAIChunk := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", string(escapedText))
		return &schema.StreamEvent{Content: text}, []byte(openAIChunk), nil
	}

	if event.Type == "message_stop" {
		return &schema.StreamEvent{Content: "[DONE]"}, []byte("data: [DONE]\n\n"), nil
	}

	// For ping, message_start, message_delta without text, return empty event without emitting bytes
	return &schema.StreamEvent{}, nil, nil
}
