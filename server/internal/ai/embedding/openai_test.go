package embedding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/ai/retry"
)

func TestOpenAIProviderRetriesAndReturnsVector(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1, 0.2}}},
		})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(Config{
		BaseURL:    server.URL,
		APIKey:     "test",
		Model:      "text-embedding-3-small",
		HTTPClient: server.Client(),
		Retry:      retry.Policy{MaxAttempts: 3, Backoff: func(int) {}, IsRetryable: retry.DefaultRetryable},
	})

	vector, err := provider.Embed(context.Background(), "hello")
	require.NoError(t, err)
	require.Equal(t, []float32{0.1, 0.2}, vector)
	require.Equal(t, 3, attempts)
}
