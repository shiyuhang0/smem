package recall

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/ai/llm"
	"smem/apps/server/internal/domain/memory"
)

func TestRecallUsesVectorDistanceDuringRerank(t *testing.T) {
	now := time.Now().UTC()
	repo := &recallRepo{
		vectorCandidates: []memory.RecallCandidate{
			{
				Memory:         memory.Memory{ID: "near", Content: "remember vim", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now},
				VectorDistance: floatPtr(0.05),
			},
			{
				Memory:         memory.Memory{ID: "far", Content: "remember vim", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now},
				VectorDistance: floatPtr(0.35),
			},
		},
		fullTextCandidates: []memory.RecallCandidate{},
	}
	svc := NewService(repo, fakeEmbedder{}, fakeLLMProvider{})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "vim", TopK: 2, Temperature: 1})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, "near", results[0].Memory.ID)
}

func TestRecallReturnsErrorWhenFullTextSearchFails(t *testing.T) {
	now := time.Now().UTC()
	repo := &recallRepo{
		vectorCandidates: []memory.RecallCandidate{{
			Memory:         memory.Memory{ID: "a", Content: "remember vim", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now},
			VectorDistance: floatPtr(0.1),
		}},
		fullTextErr: context.DeadlineExceeded,
	}
	svc := NewService(repo, fakeEmbedder{}, fakeLLMProvider{})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "vim", TopK: 3, Temperature: 1})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, results)
}

func TestRecallMergesVectorAndFullTextViaRRFAndReranks(t *testing.T) {
	now := time.Now().UTC()
	repo := &recallRepo{
		vectorCandidates: []memory.RecallCandidate{
			{Memory: memory.Memory{ID: "vec-only", Content: "vector only", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}, VectorDistance: floatPtr(0.05)},
			{Memory: memory.Memory{ID: "shared", Content: "shared", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}, VectorDistance: floatPtr(0.1)},
		},
		fullTextCandidates: []memory.RecallCandidate{
			{Memory: memory.Memory{ID: "shared", Content: "shared", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}, FullTextScore: floatPtr(0.8)},
			{Memory: memory.Memory{ID: "fts-only", Content: "fts only", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}, FullTextScore: floatPtr(1)},
		},
	}
	svc := NewService(repo, fakeEmbedder{}, fakeLLMProvider{})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "shared", TopK: 3, Temperature: 1})
	require.NoError(t, err)
	require.Len(t, results, 3)
	// Appears in both channels → strongest hybrid relevance after rerank.
	require.Equal(t, "shared", results[0].Memory.ID)
	require.ElementsMatch(t, []string{"vec-only", "shared", "fts-only"}, []string{
		results[0].Memory.ID,
		results[1].Memory.ID,
		results[2].Memory.ID,
	})
}

type recallRepo struct {
	vectorCandidates   []memory.RecallCandidate
	fullTextCandidates []memory.RecallCandidate
	fullTextErr        error
}

func (r *recallRepo) Create(_ context.Context, _ memory.Memory) (memory.Memory, error) {
	panic("unused")
}
func (r *recallRepo) UpsertByContentHash(_ context.Context, _ memory.Memory) (memory.Memory, error) {
	panic("unused")
}
func (r *recallRepo) Update(_ context.Context, _ memory.Memory) (memory.Memory, error) {
	panic("unused")
}
func (r *recallRepo) Delete(_ context.Context, _ string) error                   { panic("unused") }
func (r *recallRepo) GetByID(_ context.Context, _ string) (memory.Memory, error) { panic("unused") }

func (r *recallRepo) List(_ context.Context, _ memory.ListInput) ([]memory.Memory, int64, error) {
	panic("unused")
}

func (r *recallRepo) ListTopKinds(_ context.Context, _ int) ([]memory.KindCount, error) {
	panic("unused")
}

func (r *recallRepo) Search(_ context.Context, _ string, _ int) ([]memory.Memory, error) {
	panic("unused")
}

func (r *recallRepo) VectorSearch(_ context.Context, _ []float32, _ int) ([]memory.RecallCandidate, error) {
	return r.vectorCandidates, nil
}

func (r *recallRepo) FullTextSearch(_ context.Context, query string, _ int) ([]memory.RecallCandidate, error) {
	if r.fullTextErr != nil {
		return nil, r.fullTextErr
	}
	_ = query
	return r.fullTextCandidates, nil
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

func floatPtr(value float64) *float64 {
	return &value
}
