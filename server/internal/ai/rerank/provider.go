package rerank

import (
	"context"
	"net/http"

	"smem/apps/server/internal/ai/retry"
)

type Provider interface {
	Rerank(context.Context, string, []string, int) ([]Result, error)
}

type Result struct {
	Index          int
	RelevanceScore float64
	Document       string
}

type Config struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
	Retry      retry.Policy
}
