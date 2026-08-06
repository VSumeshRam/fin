package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
	}
}

type TextRequest struct {
	Text string `json:"text"`
}

type EmbedResponse struct {
	Vector []float64 `json:"vector"`
}

type NERResponse struct {
	Entities []string `json:"entities"`
}

func (c *Client) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	reqBody, _ := json.Marshal(TextRequest{Text: text})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/embed", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar error: status %d", resp.StatusCode)
	}

	var res EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Vector, nil
}

func (c *Client) GetEntities(ctx context.Context, text string) ([]string, error) {
	reqBody, _ := json.Marshal(TextRequest{Text: text})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/ner", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sidecar error: status %d", resp.StatusCode)
	}

	var res NERResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return res.Entities, nil
}

// CosineSimilarity computes (dot_product) / (magnitude_a * magnitude_b)
func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
