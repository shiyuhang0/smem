package rerank

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"smem/apps/server/internal/ai/retry"
)

type SiliconFlowProvider struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	retry      retry.Policy
}

func NewSiliconFlowProvider(cfg Config) *SiliconFlowProvider {
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	if cfg.Retry.MaxAttempts == 0 {
		cfg.Retry = retry.DefaultPolicy()
	}
	return &SiliconFlowProvider{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey, model: cfg.Model, httpClient: client, retry: cfg.Retry}
}

func (p *SiliconFlowProvider) Rerank(ctx context.Context, query string, documents []string, topN int) ([]Result, error) {
	if len(documents) == 0 {
		return nil, nil
	}
	body, err := p.marshalRerankRequest(query, documents, topN)
	if err != nil {
		return nil, err
	}

	payload, err := p.doRerankRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	return decodeSiliconFlowRerank(payload)
}

func (p *SiliconFlowProvider) marshalRerankRequest(query string, documents []string, topN int) ([]byte, error) {
	if topN <= 0 || topN > len(documents) {
		topN = len(documents)
	}
	return json.Marshal(map[string]any{
		"model":            p.model,
		"query":            query,
		"documents":        documents,
		"return_documents": true,
		"top_n":            topN,
	})
}

func (p *SiliconFlowProvider) doRerankRequest(ctx context.Context, body []byte) (siliconFlowRerankResponse, error) {
	var payload siliconFlowRerankResponse
	err := p.retry.Do(ctx, func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/rerank", bytes.NewReader(body))
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
		return siliconFlowRerankResponse{}, err
	}
	return payload, nil
}

type siliconFlowRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       struct {
			Text string `json:"text"`
		} `json:"document"`
	} `json:"results"`
}

func decodeSiliconFlowRerank(payload siliconFlowRerankResponse) ([]Result, error) {
	if len(payload.Results) == 0 {
		return nil, fmt.Errorf("rerank response has no results")
	}
	results := make([]Result, 0, len(payload.Results))
	for _, item := range payload.Results {
		results = append(results, Result{Index: item.Index, RelevanceScore: item.RelevanceScore, Document: item.Document.Text})
	}
	return results, nil
}
