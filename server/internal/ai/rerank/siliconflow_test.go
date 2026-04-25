package rerank

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/ai/retry"
)

func TestSiliconFlowProviderRetriesAndReturnsResults(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		require.Equal(t, "/rerank", r.URL.Path)
		require.Equal(t, "Bearer test", r.Header.Get("Authorization"))

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "BAAI/bge-reranker-v2-m3", body["model"])
		require.Equal(t, "Apple", body["query"])
		require.Equal(t, true, body["return_documents"])
		require.EqualValues(t, 2, body["top_n"])

		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"index": 1, "relevance_score": 0.91, "document": map[string]any{"text": "banana"}},
				{"index": 0, "relevance_score": 0.88, "document": map[string]any{"text": "apple"}},
			},
		})
	}))
	defer server.Close()

	provider := NewSiliconFlowProvider(Config{
		BaseURL:    server.URL,
		APIKey:     "test",
		Model:      "BAAI/bge-reranker-v2-m3",
		HTTPClient: server.Client(),
		Retry:      retry.Policy{MaxAttempts: 3, Backoff: func(int) {}, IsRetryable: retry.DefaultRetryable},
	})

	results, err := provider.Rerank(context.Background(), "Apple", []string{"apple", "banana"}, 2)
	require.NoError(t, err)
	require.Equal(t, 3, attempts)
	require.Equal(t, []Result{{Index: 1, RelevanceScore: 0.91, Document: "banana"}, {Index: 0, RelevanceScore: 0.88, Document: "apple"}}, results)
}

func TestSiliconFlowProviderRejectsEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer server.Close()

	provider := NewSiliconFlowProvider(Config{BaseURL: server.URL, APIKey: "test", Model: "BAAI/bge-reranker-v2-m3", HTTPClient: server.Client()})

	results, err := provider.Rerank(context.Background(), "Apple", []string{"apple"}, 1)
	require.Nil(t, results)
	require.ErrorContains(t, err, "no results")
}
