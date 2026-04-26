package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"smem/apps/server/internal/ai/retry"
)

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	Dimensions int
	HTTPClient *http.Client
	Retry      retry.Policy
}

type OpenAIProvider struct {
	baseURL    string
	apiKey     string
	model      string
	dimensions int
	httpClient *http.Client
	retry      retry.Policy
}

func NewOpenAIProvider(cfg Config) *OpenAIProvider {
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry = retry.DefaultPolicy()
	}
	return &OpenAIProvider{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		httpClient: client,
		retry:      cfg.Retry,
	}
}

func (p *OpenAIProvider) Embed(ctx context.Context, input string) ([]float32, error) {
	body, err := p.marshalEmbeddingRequest(input)
	if err != nil {
		return nil, err
	}

	payload, err := p.doEmbeddingRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	return decodeOpenAIEmbedding(payload)
}

func (p *OpenAIProvider) marshalEmbeddingRequest(input string) ([]byte, error) {
	body := map[string]any{"model": p.model, "input": input}
	if p.dimensions > 0 {
		body["dimensions"] = p.dimensions
	}
	return json.Marshal(body)
}

func (p *OpenAIProvider) doEmbeddingRequest(ctx context.Context, body []byte) (openAIEmbeddingResponse, error) {
	var payload openAIEmbeddingResponse
	err := p.retry.Do(ctx, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/embeddings", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return retry.HTTPStatusError{StatusCode: resp.StatusCode}
		}

		return json.NewDecoder(resp.Body).Decode(&payload)
	})
	if err != nil {
		return openAIEmbeddingResponse{}, err
	}
	return payload, nil
}

type openAIEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func decodeOpenAIEmbedding(payload openAIEmbeddingResponse) ([]float32, error) {
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("embedding response has no data")
	}
	return payload.Data[0].Embedding, nil
}
