package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/finops-gateway/internal/schema"
)

type OpenAIAdapter struct{}

func (a *OpenAIAdapter) FormatRequest(payload *schema.StandardPayload) ([]byte, error) {
	return json.Marshal(payload)
}

func (a *OpenAIAdapter) FormatResponse(responseBody []byte) ([]byte, error) {
	return responseBody, nil
}

func (a *OpenAIAdapter) TargetPath() string {
	return "/v1/chat/completions"
}

func (a *OpenAIAdapter) PrepareRequest(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
}

// openaiResponse matches the standard OpenAI usage payload
type openaiResponse struct {
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (a *OpenAIAdapter) ParseUsage(responseBody []byte) (*schema.TokenUsage, error) {
	var resp openaiResponse
	err := json.Unmarshal(responseBody, &resp)
	if err != nil {
		return nil, err
	}
	
	if resp.Model == "" {
		return nil, fmt.Errorf("missing model in response")
	}

	return &schema.TokenUsage{
		Model:            resp.Model,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}, nil
}

type openaiStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// TranslateChunk satisfies adapter.StreamTranslator
func (a *OpenAIAdapter) TranslateChunk(chunk []byte) (*schema.StreamEvent, []byte, error) {
	trimmed := bytes.TrimSpace(chunk)
	if !bytes.HasPrefix(trimmed, []byte("data:")) {
		return nil, chunk, fmt.Errorf("chunk is not an SSE data line")
	}

	payload := bytes.TrimSpace(bytes.TrimPrefix(trimmed, []byte("data:")))
	if bytes.Equal(payload, []byte("[DONE]")) {
		return &schema.StreamEvent{Content: "[DONE]"}, chunk, nil
	}

	var sc openaiStreamChunk
	if err := json.Unmarshal(payload, &sc); err != nil {
		return nil, chunk, err
	}

	var content string
	if len(sc.Choices) > 0 {
		content = sc.Choices[0].Delta.Content
	}

	return &schema.StreamEvent{Content: content}, chunk, nil
}