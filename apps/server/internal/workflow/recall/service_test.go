package recall

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/llm"
)

func TestRecallFallsBackToRawQueryAndReturnsActiveMemories(t *testing.T) {
	repo := &recallRepo{items: []memory.Memory{
		{ID: "a", Content: "remember vim", State: memory.StateActive, Type: memory.TypeFact, Kinds: []string{"preference"}, StoreCount: 3, UpdatedAt: time.Now().UTC()},
		{ID: "b", Content: "draft note", State: memory.StateCreating},
	}}
	svc := NewService(repo, fakeEmbedder{}, fakeLLMProvider{err: context.DeadlineExceeded})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "vim", TopK: 3, Temperature: 1})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "a", results[0].Memory.ID)
}

func TestRecallUsesRewriteTypeAndKindsToBoostMatches(t *testing.T) {
	now := time.Now().UTC()
	repo := &recallRepo{items: []memory.Memory{
		{ID: "a", Content: "editor preference", State: memory.StateActive, Type: memory.TypeFact, Kinds: []string{"note"}, StoreCount: 3, UpdatedAt: now},
		{ID: "b", Content: "editor preference", State: memory.StateActive, Type: memory.TypeFact, Kinds: []string{"preference"}, StoreCount: 1, UpdatedAt: now},
	}}
	svc := NewService(repo, fakeEmbedder{}, fakeLLMProvider{response: `{"content":"editor preference","type":"fact","kinds":["preference"]}`})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "what editor do i use", TopK: 2, Temperature: 1})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "b", results[0].Memory.ID)
}

type recallRepo struct{ items []memory.Memory }

func (r *recallRepo) Create(context.Context, memory.Memory) (memory.Memory, error) { panic("unused") }
func (r *recallRepo) Update(context.Context, memory.Memory) (memory.Memory, error) { panic("unused") }
func (r *recallRepo) Delete(context.Context, string) error                         { panic("unused") }
func (r *recallRepo) GetByID(context.Context, string) (memory.Memory, error)       { panic("unused") }
func (r *recallRepo) GetByContentHash(context.Context, string) (memory.Memory, error) {
	panic("unused")
}
func (r *recallRepo) List(_ context.Context, input memory.ListInput) ([]memory.Memory, int64, error) {
	filtered := make([]memory.Memory, 0, len(r.items))
	for _, item := range r.items {
		if input.State != "" && item.State != input.State {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, int64(len(filtered)), nil
}
func (r *recallRepo) Search(_ context.Context, query string, _ int) ([]memory.Memory, error) {
	out := make([]memory.Memory, 0, len(r.items))
	for _, item := range r.items {
		if item.State == memory.StateActive && query != "" {
			out = append(out, item)
		}
	}
	return out, nil
}

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.5, 0.1}, nil
}

type fakeLLMProvider struct {
	response string
	err      error
}

func (f fakeLLMProvider) GenerateText(context.Context, []llm.Message) (string, error) {
	return f.response, f.err
}
