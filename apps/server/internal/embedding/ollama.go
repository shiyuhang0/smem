package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"smem/apps/server/internal/retry"
)

type OllamaProvider struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	retry      retry.Policy
}

func NewOllamaProvider(cfg Config) *OllamaProvider {
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry = retry.DefaultPolicy()
	}
	return &OllamaProvider{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey, model: cfg.Model, httpClient: client, retry: cfg.Retry}
}

func (p *OllamaProvider) Embed(ctx context.Context, input string) ([]float32, error) {
	body, err := json.Marshal(map[string]any{"model": p.model, "input": input})
	if err != nil {
		return nil, err
	}
	var vector []float32
	err = p.retry.Do(ctx, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/embed", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}
		resp, err := p.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return retry.HTTPStatusError{StatusCode: resp.StatusCode}
		}
		var payload struct {
			Embedding  []float64   `json:"embedding"`
			Embeddings [][]float64 `json:"embeddings"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return err
		}
		switch {
		case len(payload.Embeddings) > 0:
			vector = float64ToFloat32(payload.Embeddings[0])
		case len(payload.Embedding) > 0:
			vector = float64ToFloat32(payload.Embedding)
		default:
			return fmt.Errorf("embedding response has no data")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return vector, nil
}

func float64ToFloat32(values []float64) []float32 {
	result := make([]float32, len(values))
	for i, value := range values {
		result[i] = float32(value)
	}
	return result
}
