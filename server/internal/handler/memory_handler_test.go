package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"
)

func TestMemoryHandlerCreateReturnsIngestJob(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newHandlerRepo()
	svc := memory.NewService(repo)
	h := NewMemoryHandler(svc, fakeIngestService{
		job: ingestjob.Job{
			ID:           "job-1",
			State:        ingestjob.StatePending,
			Mode:         ingestjob.ModeNormal,
			ExecuteCount: 0,
			CreatedAt:    time.Unix(100, 0).UTC(),
			UpdatedAt:    time.Unix(100, 0).UTC(),
		},
	}, nil)
	r := gin.New()
	h.Register(r.Group("/api/v1"))

	body, _ := json.Marshal(map[string]any{"content": "remember this", "mode": "normal"})
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/memories", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	r.ServeHTTP(createResp, createReq)
	require.Equal(t, http.StatusAccepted, createResp.Code)
	require.Contains(t, createResp.Body.String(), `"id":"job-1"`)
	require.Contains(t, createResp.Body.String(), `"state":"pending"`)
}

func TestMemoryHandlerGetExistingMemory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newHandlerRepo()
	now := time.Unix(100, 0).UTC()
	repo.items["memory-1"] = memory.Memory{
		ID:          "memory-1",
		Content:     "remember this",
		ContentHash: memory.HashContent("remember this"),
		Kind:        "note",
		Scope:       memory.ScopeUser,
		State:       memory.StateActive,
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc := memory.NewService(repo)
	h := NewMemoryHandler(svc, nil, nil)
	r := gin.New()
	h.Register(r.Group("/api/v1"))

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/memories/memory-1", nil)
	getResp := httptest.NewRecorder()
	r.ServeHTTP(getResp, getReq)
	require.Equal(t, http.StatusOK, getResp.Code)
	require.Contains(t, getResp.Body.String(), "remember this")
	require.Contains(t, getResp.Body.String(), `"kind":"note"`)
}

func TestMemoryHandlerCreateReturnsInternalServerErrorForIngestFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newHandlerRepo()
	svc := memory.NewService(repo)
	h := NewMemoryHandler(svc, fakeIngestService{err: errors.New("db down")}, nil)
	r := gin.New()
	h.Register(r.Group("/api/v1"))

	body, _ := json.Marshal(map[string]any{"content": "remember this", "mode": "normal"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memories", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusInternalServerError, resp.Code)
}

type fakeIngestService struct {
	job ingestjob.Job
	err error
}

func (f fakeIngestService) Create(_ context.Context, _ memory.CreateInput) (ingestjob.Job, error) {
	return f.job, f.err
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

func (r *handlerRepo) UpsertByContentHash(_ context.Context, m memory.Memory) (memory.Memory, error) {
	for id, item := range r.items {
		if item.ContentHash != m.ContentHash {
			continue
		}
		item.StoreCount += m.StoreCount
		item.UpdatedAt = m.UpdatedAt
		r.items[id] = item
		return item, nil
	}
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

func (r *handlerRepo) List(_ context.Context, _ memory.ListInput) ([]memory.Memory, int64, error) {
	out := make([]memory.Memory, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, int64(len(out)), nil
}

func (r *handlerRepo) ListTopKinds(_ context.Context, _ int) ([]memory.KindCount, error) {
	return nil, nil
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
