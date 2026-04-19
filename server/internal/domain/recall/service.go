package recall

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"smem/apps/server/internal/ai/embedding"
	"smem/apps/server/internal/ai/llm"
	"smem/apps/server/internal/config"
	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/domain/recall/scoring"
)

var recallLogger = config.NewLogger("[recall] ")

const enabelSoftmax = false

type Service struct {
	repo      memory.Repository
	embedder  embedding.Provider
	llm       llm.Provider
	randFloat func() float64
}

func NewService(repo memory.Repository, embedder embedding.Provider, llmProvider llm.Provider) *Service {
	return &Service{
		repo:      repo,
		embedder:  embedder,
		llm:       llmProvider,
		randFloat: rand.Float64,
	}
}

func (s *Service) Recall(ctx context.Context, input memory.RecallInput) ([]memory.RecallResult, error) {
	input = normalizeRecallInput(input)

	query := rewriteRecallQuery(input)
	candidates, err := s.loadRecallCandidates(ctx, query, input.TopK)
	if err != nil {
		return nil, err
	}

	results := s.rerankRecallResults(candidates, time.Now().UTC())
	if len(results) == 0 {
		return nil, nil
	}

	return s.finalizeRecallResults(results, input), nil
}

type rewriteResult struct {
	Content string   `json:"content"`
	Type    string   `json:"type"`
	Kinds   []string `json:"kinds"`
}

func normalizeRecallInput(input memory.RecallInput) memory.RecallInput {
	if input.TopK == 0 {
		input.TopK = 5
	}
	if input.Temperature == 0 {
		input.Temperature = 1
	}
	return input
}

func rewriteRecallQuery(input memory.RecallInput) string {
	// Keep the placeholder rewrite stage explicit so content/type/kinds can be added later.
	rewritten := rewriteResult{Content: input.Content}
	return rewritten.Content
}

func (s *Service) loadRecallCandidates(ctx context.Context, query string, topK int) ([]memory.RecallCandidate, error) {
	vectorCandidates, err := s.searchVectorCandidates(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	recallLogger.Printf("vector_search_results=%v", summarizeRecallCandidates(vectorCandidates))

	fullTextCandidates, err := s.searchFullTextCandidates(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	recallLogger.Printf("full_search_results=%v", summarizeRecallCandidates(fullTextCandidates))

	rrfCandidates := s.rrfCandidates(vectorCandidates, fullTextCandidates, topK)
	recallLogger.Printf("rrf_candidates=%v", summarizeRecallCandidates(rrfCandidates))
	return rrfCandidates, nil
}

func (s *Service) rerankRecallResults(candidates []memory.RecallCandidate, now time.Time) []memory.RecallResult {
	results := s.rerankCandidates(candidates, "hybrid_recall", now)
	recallLogger.Printf("final_scores=%v", summarizeRecallResults(results))
	return results
}

func (s *Service) finalizeRecallResults(results []memory.RecallResult, input memory.RecallInput) []memory.RecallResult {
	// Softmax can surface weakly related candidates, so keep it behind the explicit flag.
	if enabelSoftmax {
		results = s.applySoftmaxScores(results, input.Temperature)
		recallLogger.Printf("final_probabilities=%v", summarizeRecallResults(results))
		return s.selectTopKByProbability(results, input.TopK)
	}

	return results[:input.TopK]
}

func (s *Service) searchVectorCandidates(ctx context.Context, query string, topK int) ([]memory.RecallCandidate, error) {
	if s.embedder == nil {
		return nil, nil
	}
	vector, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, nil
	}
	return s.repo.VectorSearch(ctx, vector, topK*2)
}

func (s *Service) searchFullTextCandidates(ctx context.Context, query string, topK int) ([]memory.RecallCandidate, error) {
	return s.repo.FullTextSearch(ctx, query, topK*2)
}

func (s *Service) rrfCandidates(vectorCandidates, fullTextCandidates []memory.RecallCandidate, topK int) []memory.RecallCandidate {
	vectorIDs := extractIDs(vectorCandidates)
	fullTextIDs := extractIDs(fullTextCandidates)
	rankedIDs := scoring.RRF([][]string{vectorIDs, fullTextIDs}, 60)
	if len(rankedIDs) > topK*2 {
		rankedIDs = rankedIDs[:topK*2]
	}
	mergedCandidates := mergeCandidatesByID(vectorCandidates, fullTextCandidates)
	return orderCandidatesByIDs(rankedIDs, mergedCandidates)
}

func (s *Service) rerankCandidates(candidates []memory.RecallCandidate, reason string, now time.Time) []memory.RecallResult {
	maxStoreCount := maxCandidateStoreCount(candidates)
	maxFullTextScore := maxCandidateFullTextScore(candidates)
	results := make([]memory.RecallResult, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Memory.Searchable() {
			continue
		}
		score := scoring.Score(scoring.ScoreInput{
			Candidate:        candidate,
			Now:              now,
			MaxStoreCount:    maxStoreCount,
			MaxFullTextScore: maxFullTextScore,
		})
		results = append(results, memory.RecallResult{Memory: candidate.Memory, Score: score, Reason: reason})
	}
	return results
}

func (s *Service) applySoftmaxScores(results []memory.RecallResult, temperature float64) []memory.RecallResult {
	rawScores := make([]float64, 0, len(results))
	for _, result := range results {
		rawScores = append(rawScores, result.Score)
	}
	probabilities := scoring.Softmax(rawScores, temperature)
	scored := append([]memory.RecallResult(nil), results...)
	for i := range scored {
		scored[i].Score = probabilities[i]
	}
	return scored
}

func extractIDs(items []memory.RecallCandidate) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Memory.ID)
	}
	return ids
}

func mergeCandidatesByID(groups ...[]memory.RecallCandidate) map[string]memory.RecallCandidate {
	byID := map[string]memory.RecallCandidate{}
	for _, group := range groups {
		for _, candidate := range group {
			existing, ok := byID[candidate.Memory.ID]
			if !ok {
				byID[candidate.Memory.ID] = candidate
				continue
			}
			if existing.VectorDistance == nil {
				existing.VectorDistance = candidate.VectorDistance
			}
			if existing.FullTextScore == nil {
				existing.FullTextScore = candidate.FullTextScore
			}
			byID[candidate.Memory.ID] = existing
		}
	}
	return byID
}

func orderCandidatesByIDs(ids []string, byID map[string]memory.RecallCandidate) []memory.RecallCandidate {
	ordered := make([]memory.RecallCandidate, 0, len(ids))
	for _, id := range ids {
		candidate, ok := byID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, candidate)
	}
	return ordered
}

func maxCandidateStoreCount(candidates []memory.RecallCandidate) int {
	maxStoreCount := 0
	for _, candidate := range candidates {
		if candidate.Memory.StoreCount > maxStoreCount {
			maxStoreCount = candidate.Memory.StoreCount
		}
	}
	return maxStoreCount
}

func maxCandidateFullTextScore(candidates []memory.RecallCandidate) float64 {
	maxScore := 0.0
	for _, candidate := range candidates {
		if candidate.FullTextScore == nil {
			continue
		}
		if *candidate.FullTextScore > maxScore {
			maxScore = *candidate.FullTextScore
		}
	}
	return maxScore
}

func (s *Service) selectTopKByProbability(results []memory.RecallResult, topK int) []memory.RecallResult {
	if topK <= 0 || len(results) <= topK {
		sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
		return results
	}

	remaining := append([]memory.RecallResult(nil), results...)
	selected := make([]memory.RecallResult, 0, topK)
	for len(selected) < topK && len(remaining) > 0 {
		selectedIndex := pickByProbability(remaining, s.randFloat())
		selected = append(selected, remaining[selectedIndex])
		remaining = append(remaining[:selectedIndex], remaining[selectedIndex+1:]...)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Score > selected[j].Score })
	return selected
}

func pickByProbability(results []memory.RecallResult, draw float64) int {
	total := 0.0
	for _, result := range results {
		total += result.Score
	}
	if total <= 0 {
		return 0
	}
	threshold := draw * total
	cumulative := 0.0
	for i, result := range results {
		cumulative += result.Score
		if threshold <= cumulative {
			return i
		}
	}
	return len(results) - 1
}

func summarizeRecallResults(results []memory.RecallResult) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, contentSnippet(result.Memory.Content)+"("+result.Memory.ID+")"+":"+formatFloat(result.Score))
	}
	return out
}

func summarizeRecallCandidates(candidates []memory.RecallCandidate) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out,
			candidate.Memory.ID+":"+
				contentSnippet(candidate.Memory.Content)+
				":distance="+formatOptionalFloat(candidate.VectorDistance)+
				":score="+formatOptionalFloat(candidate.FullTextScore),
		)
	}
	return out
}

func contentSnippet(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\n", " ")
	const maxLen = 80
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "..."
}

func formatFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 6, 64), "0"), ".")
}

func formatOptionalFloat(value *float64) string {
	if value == nil {
		return "nil"
	}
	return formatFloat(*value)
}
