package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/retry"
)

func TestOpenAIProviderRetriesAndReturnsMessage(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"content": "rewritten query"},
			}},
		})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(Config{
		BaseURL:    server.URL,
		APIKey:     "test",
		Model:      "gpt-4.1-mini",
		HTTPClient: server.Client(),
		Retry:      retry.Policy{MaxAttempts: 3, Backoff: func(int) {}, IsRetryable: retry.DefaultRetryable},
	})

	text, err := provider.GenerateText(context.Background(), []Message{{Role: "user", Content: "hello"}})
	require.NoError(t, err)
	require.Equal(t, "rewritten query", text)
	require.Equal(t, 3, attempts)
}
