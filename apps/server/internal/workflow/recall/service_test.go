package recall

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/llm"
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

func TestRecallUsesRRFToMergeVectorAndFullTextCandidates(t *testing.T) {
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
	require.ElementsMatch(t, []string{"vec-only", "shared", "fts-only"}, []string{
		results[0].Memory.ID,
		results[1].Memory.ID,
		results[2].Memory.ID,
	})
}

func TestFuseRecallCandidatesUsesRRFWhenFullTextAvailable(t *testing.T) {
	vectorCandidates := []memory.RecallCandidate{
		{Memory: memory.Memory{ID: "vec-only"}},
		{Memory: memory.Memory{ID: "shared"}},
	}
	fullTextCandidates := []memory.RecallCandidate{
		{Memory: memory.Memory{ID: "shared"}},
		{Memory: memory.Memory{ID: "fts-only"}},
	}
	svc := NewService(&recallRepo{}, nil, fakeLLMProvider{})

	candidates := svc.rrfCandidates(vectorCandidates, fullTextCandidates, 2)

	require.Len(t, candidates, 3)
	require.Equal(t, "shared", candidates[0].Memory.ID)
	require.Equal(t, "vec-only", candidates[1].Memory.ID)
	require.Equal(t, "fts-only", candidates[2].Memory.ID)
}

func TestApplySoftmaxScoresReturnsProbabilities(t *testing.T) {
	svc := NewService(&recallRepo{}, nil, fakeLLMProvider{})
	results := []memory.RecallResult{
		{Memory: memory.Memory{ID: "a"}, Score: 2},
		{Memory: memory.Memory{ID: "b"}, Score: 1},
	}

	probabilities := svc.applySoftmaxScores(results, 1)

	require.Len(t, probabilities, 2)
	require.Greater(t, probabilities[0].Score, probabilities[1].Score)
	require.InDelta(t, 1.0, probabilities[0].Score+probabilities[1].Score, 0.000001)
}

func TestSummarizeRecallCandidatesOnlyPrintsKeyFields(t *testing.T) {
	candidates := []memory.RecallCandidate{
		{
			Memory:         memory.Memory{ID: "a", Content: "remember vim and tmux"},
			VectorDistance: floatPtr(0.12),
			FullTextScore:  floatPtr(0.8),
		},
	}

	summary := summarizeRecallCandidates(candidates)

	require.Equal(t, []string{"a:remember vim and tmux:distance=0.12:score=0.8"}, summary)
}

func TestSelectTopKByProbabilityCanChooseLowerProbabilityResult(t *testing.T) {
	results := []memory.RecallResult{
		{Memory: memory.Memory{ID: "a"}, Score: 0.6},
		{Memory: memory.Memory{ID: "b"}, Score: 0.3},
		{Memory: memory.Memory{ID: "c"}, Score: 0.1},
	}
	draws := []float64{0.95, 0.1}
	svc := NewService(&recallRepo{}, nil, fakeLLMProvider{})
	svc.randFloat = func() float64 {
		value := draws[0]
		draws = draws[1:]
		return value
	}

	selected := svc.selectTopKByProbability(results, 2)

	require.Len(t, selected, 2)
	require.Equal(t, "a", selected[0].Memory.ID)
	require.Equal(t, "c", selected[1].Memory.ID)
}

type recallRepo struct {
	vectorCandidates   []memory.RecallCandidate
	fullTextCandidates []memory.RecallCandidate
	fullTextErr        error
}

func (r *recallRepo) Create(context.Context, memory.Memory) (memory.Memory, error) { panic("unused") }
func (r *recallRepo) Update(context.Context, memory.Memory) (memory.Memory, error) { panic("unused") }
func (r *recallRepo) Delete(context.Context, string) error                         { panic("unused") }
func (r *recallRepo) GetByID(context.Context, string) (memory.Memory, error)       { panic("unused") }
func (r *recallRepo) GetByContentHash(context.Context, string) (memory.Memory, error) {
	panic("unused")
}
func (r *recallRepo) List(_ context.Context, _ memory.ListInput) ([]memory.Memory, int64, error) {
	return nil, 0, nil
}
func (r *recallRepo) Search(_ context.Context, query string, _ int) ([]memory.Memory, error) {
	_ = query
	return nil, nil
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
