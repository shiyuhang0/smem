package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/llm"
)

func TestNormalCreateQueuesEmbeddingAndActivatesMemory(t *testing.T) {
	repo := newRepo()
	memSvc := memory.NewService(repo, func() string { return "memory-1" })
	worker := &fakeWorker{}
	svc := NewService(memSvc, repo, worker, nil)

	items, err := svc.Create(context.Background(), memory.CreateInput{Content: "remember this", Mode: memory.ModeNormal})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, memory.StateCreating, items[0].State)
	require.Equal(t, "memory-1", worker.lastID)

	items[0].Embedding = []float32{0.1, 0.2}
	items[0].State = memory.StateActive
	_, err = repo.Update(context.Background(), items[0])
	require.NoError(t, err)

	loaded, err := repo.GetByID(context.Background(), "memory-1")
	require.NoError(t, err)
	require.Equal(t, memory.StateActive, loaded.State)
}

func TestSmartCreateFallsBackToNormalWhenExtractionFails(t *testing.T) {
	repo := newRepo()
	memSvc := memory.NewService(repo, func() string { return "memory-1" })
	worker := &fakeWorker{}
	svc := NewService(memSvc, repo, worker, fakeLLM{err: errors.New("boom")})

	items, err := svc.Create(context.Background(), memory.CreateInput{Content: "remember this", Mode: memory.ModeSmart})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "remember this", items[0].Content)
	require.Equal(t, memory.StateCreating, items[0].State)
}

func TestSmartCreateUsesFusionDecisionToUpdateExistingMemory(t *testing.T) {
	repo := newRepo()
	existing := memory.Memory{
		ID:          "existing-1",
		Content:     "user prefers vim",
		ContentHash: memory.HashContent("user prefers vim"),
		Type:        memory.TypeFact,
		Kinds:       []string{"preference"},
		Scope:       memory.ScopeUser,
		State:       memory.StateActive,
		Version:     1,
		StoreCount:  1,
		CreatedAt:   time.Unix(10, 0).UTC(),
		UpdatedAt:   time.Unix(10, 0).UTC(),
	}
	_, _ = repo.Create(context.Background(), existing)
	memSvc := memory.NewService(repo, func() string { return "memory-2" })
	worker := &fakeWorker{}
	svc := NewService(memSvc, repo, worker, &sequenceLLM{responses: []fakeResponse{
		{text: `{"memories":[{"content":"user prefers neovim","type":"fact","kinds":["preference"]}]}`},
		{text: `{"decision":"update","memory_id":"existing-1","content":"user prefers neovim"}`},
	}})

	items, err := svc.Create(context.Background(), memory.CreateInput{Content: "I prefer neovim", Mode: memory.ModeSmart})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "existing-1", items[0].ID)
	require.Equal(t, "user prefers neovim", items[0].Content)
	require.Equal(t, 2, items[0].Version)
	require.Equal(t, 2, items[0].StoreCount)
}

func TestSmartCreateSkipsIgnoredCandidate(t *testing.T) {
	repo := newRepo()
	memSvc := memory.NewService(repo, func() string { return "memory-1" })
	worker := &fakeWorker{}
	svc := NewService(memSvc, repo, worker, &sequenceLLM{responses: []fakeResponse{
		{text: `{"memories":[{"content":"temporary typo","type":"fact","kinds":["note"]}]}`},
		{text: `{"decision":"ignore"}`},
	}})

	items, err := svc.Create(context.Background(), memory.CreateInput{Content: "temporary typo", Mode: memory.ModeSmart})
	require.NoError(t, err)
	require.Len(t, items, 0)
}

type fakeWorker struct{ lastID string }

func (w *fakeWorker) Queue(_ context.Context, item memory.Memory) error {
	w.lastID = item.ID
	return nil
}

type fakeLLM struct {
	response string
	err      error
}

func (f fakeLLM) GenerateText(_ context.Context, _ []llm.Message) (string, error) {
	return f.response, f.err
}

type fakeResponse struct {
	text string
	err  error
}

type sequenceLLM struct {
	responses []fakeResponse
	index     int
}

func (s *sequenceLLM) GenerateText(_ context.Context, _ []llm.Message) (string, error) {
	if s.index >= len(s.responses) {
		return "", errors.New("no more llm responses")
	}
	resp := s.responses[s.index]
	s.index++
	return resp.text, resp.err
}

type repo struct{ items map[string]memory.Memory }

func newRepo() *repo { return &repo{items: map[string]memory.Memory{}} }

func (r *repo) Create(_ context.Context, m memory.Memory) (memory.Memory, error) {
	r.items[m.ID] = m
	return m, nil
}
func (r *repo) Update(_ context.Context, m memory.Memory) (memory.Memory, error) {
	r.items[m.ID] = m
	return m, nil
}
func (r *repo) Delete(_ context.Context, id string) error { delete(r.items, id); return nil }
func (r *repo) GetByID(_ context.Context, id string) (memory.Memory, error) {
	m, ok := r.items[id]
	if !ok {
		return memory.Memory{}, memory.ErrNotFound
	}
	return m, nil
}
func (r *repo) GetByContentHash(_ context.Context, hash string) (memory.Memory, error) {
	for _, item := range r.items {
		if item.ContentHash == hash {
			return item, nil
		}
	}
	return memory.Memory{}, memory.ErrNotFound
}
func (r *repo) List(_ context.Context, input memory.ListInput) ([]memory.Memory, int64, error) {
	out := make([]memory.Memory, 0, len(r.items))
	for _, item := range r.items {
		if input.State != "" && item.State != input.State {
			continue
		}
		out = append(out, item)
	}
	return out, int64(len(out)), nil
}
func (r *repo) Search(_ context.Context, _ string, _ int) ([]memory.Memory, error) { return nil, nil }

func init() { _ = time.UTC }
