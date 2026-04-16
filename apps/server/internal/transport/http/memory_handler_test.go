package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/memory"
)

func TestMemoryHandlerCreateAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newHandlerRepo()
	svc := memory.NewService(repo, func() string { return "memory-1" })
	h := NewMemoryHandler(svc, nil, nil)
	r := gin.New()
	h.Register(r.Group("/api/v1"))

	body, _ := json.Marshal(map[string]any{"content": "remember this", "mode": "normal"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/memories", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	r.ServeHTTP(createResp, createReq)
	require.Equal(t, http.StatusAccepted, createResp.Code)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/memories/memory-1", nil)
	getResp := httptest.NewRecorder()
	r.ServeHTTP(getResp, getReq)
	require.Equal(t, http.StatusOK, getResp.Code)
	require.Contains(t, getResp.Body.String(), "remember this")
}

type handlerRepo struct {
	items map[string]memory.Memory
}

func newHandlerRepo() *handlerRepo {
	return &handlerRepo{items: map[string]memory.Memory{}}
}

func (r *handlerRepo) Create(_ context.Context, m memory.Memory) (memory.Memory, error) {
	r.items[m.ID] = m
	return m, nil
}

func (r *handlerRepo) Update(_ context.Context, m memory.Memory) (memory.Memory, error) {
	r.items[m.ID] = m
	return m, nil
}

func (r *handlerRepo) Delete(_ context.Context, id string) error {
	delete(r.items, id)
	return nil
}

func (r *handlerRepo) GetByID(_ context.Context, id string) (memory.Memory, error) {
	m, ok := r.items[id]
	if !ok {
		return memory.Memory{}, memory.ErrNotFound
	}
	return m, nil
}

func (r *handlerRepo) GetByContentHash(_ context.Context, hash string) (memory.Memory, error) {
	for _, item := range r.items {
		if item.ContentHash == hash {
			return item, nil
		}
	}
	return memory.Memory{}, memory.ErrNotFound
}

func (r *handlerRepo) List(_ context.Context, _ memory.ListInput) ([]memory.Memory, int64, error) {
	out := make([]memory.Memory, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, int64(len(out)), nil
}

func (r *handlerRepo) Search(_ context.Context, _ string, _ int) ([]memory.Memory, error) {
	return nil, nil
}

func (r *handlerRepo) VectorSearch(_ context.Context, _ []float32, _ int) ([]memory.RecallCandidate, error) {
	return nil, nil
}

func (r *handlerRepo) FullTextSearch(_ context.Context, _ string, _ int) ([]memory.RecallCandidate, error) {
	return nil, nil
}

func init() {
	gin.SetMode(gin.TestMode)
	_ = time.UTC
}
