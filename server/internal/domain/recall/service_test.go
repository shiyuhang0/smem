package recall

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/ai/rerank"
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
	svc := NewService(repo, fakeEmbedder{}, fakeReranker{results: []rerank.Result{{Index: 0, RelevanceScore: 0.9}, {Index: 1, RelevanceScore: 0.7}}})

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
	svc := NewService(repo, fakeEmbedder{}, fakeReranker{})

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
	svc := NewService(repo, fakeEmbedder{}, fakeReranker{results: []rerank.Result{{Index: 1, RelevanceScore: 0.95}, {Index: 0, RelevanceScore: 0.8}, {Index: 2, RelevanceScore: 0.75}}})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "shared", TopK: 3, Temperature: 1})
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, "shared", results[0].Memory.ID)
	require.ElementsMatch(t, []string{"vec-only", "shared", "fts-only"}, []string{results[0].Memory.ID, results[1].Memory.ID, results[2].Memory.ID})
}

func TestRecallUsesFourXTopKSearchDepthAndHeadProtection(t *testing.T) {
	now := time.Now().UTC()
	vectorCandidates := make([]memory.RecallCandidate, 0, 20)
	fullTextCandidates := make([]memory.RecallCandidate, 0, 20)
	for i := range 20 {
		vectorCandidates = append(vectorCandidates, memory.RecallCandidate{Memory: memory.Memory{ID: fmt.Sprintf("vec-%02d", i), Content: fmt.Sprintf("vector %02d", i), State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}})
		fullTextCandidates = append(fullTextCandidates, memory.RecallCandidate{Memory: memory.Memory{ID: fmt.Sprintf("fts-%02d", i), Content: fmt.Sprintf("fts %02d", i), State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}})
	}
	repo := &recallRepo{vectorCandidates: vectorCandidates, fullTextCandidates: fullTextCandidates}
	rerankResults := make([]rerank.Result, 0, 10)
	for i := range 10 {
		rerankResults = append(rerankResults, rerank.Result{Index: i, RelevanceScore: 0.9 - float64(i)*0.01})
	}
	svc := NewService(repo, fakeEmbedder{}, fakeReranker{results: rerankResults})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "query", TopK: 5, Temperature: 1})
	require.NoError(t, err)
	require.Equal(t, 20, repo.vectorLimit)
	require.Equal(t, 20, repo.fullTextLimit)
	require.Len(t, results, 5)
	require.Equal(t, "vec-00", results[0].Memory.ID)
	require.Equal(t, "vec-04", results[4].Memory.ID)
}

func TestRecallFiltersLowRerankScores(t *testing.T) {
	now := time.Now().UTC()
	repo := &recallRepo{vectorCandidates: []memory.RecallCandidate{{Memory: memory.Memory{ID: "keep", Content: "keep", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}}, {Memory: memory.Memory{ID: "drop", Content: "drop", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}}}}
	svc := NewService(repo, fakeEmbedder{}, fakeReranker{results: []rerank.Result{{Index: 0, RelevanceScore: 0.8}, {Index: 1, RelevanceScore: 0.59}}})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "query", TopK: 5, Temperature: 1})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "keep", results[0].Memory.ID)
}

type recallRepo struct {
	vectorCandidates   []memory.RecallCandidate
	fullTextCandidates []memory.RecallCandidate
	fullTextErr        error
	vectorLimit        int
	fullTextLimit      int
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

func (r *recallRepo) VectorSearch(_ context.Context, _ []float32, limit int) ([]memory.RecallCandidate, error) {
	r.vectorLimit = limit
	return r.vectorCandidates, nil
}

func (r *recallRepo) FullTextSearch(_ context.Context, query string, limit int) ([]memory.RecallCandidate, error) {
	r.fullTextLimit = limit
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

type fakeReranker struct {
	results []rerank.Result
	err     error
}

func (f fakeReranker) Rerank(context.Context, string, []string, int) ([]rerank.Result, error) {
	return f.results, f.err
}

func floatPtr(value float64) *float64 {
	return &value
}
