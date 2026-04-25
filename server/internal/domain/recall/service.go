package recall

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"smem/apps/server/internal/ai/embedding"
	"smem/apps/server/internal/ai/rerank"
	"smem/apps/server/internal/config"
	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/domain/recall/scoring"
)

var recallLogger = config.NewLogger("[recall] ")

const (
	enabelSoftmax         = false
	searchDepthMultiplier = 4
	headSizeMultiplier    = 1
	rrfTailMultiplier     = 3
	rrfTopMultiplier      = 4
	rerankThreshold       = 0.6
)

type Service struct {
	repo      memory.Repository
	embedder  embedding.Provider
	reranker  rerank.Provider
	randFloat func() float64
}

func NewService(repo memory.Repository, embedder embedding.Provider, reranker rerank.Provider) *Service {
	return &Service{
		repo:      repo,
		embedder:  embedder,
		reranker:  reranker,
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

	results, err := s.rerankRecallResults(ctx, query, candidates)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	return s.finalizeRecallResults(results, input), nil
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
	return input.Content
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

func (s *Service) rerankRecallResults(ctx context.Context, query string, candidates []memory.RecallCandidate) ([]memory.RecallResult, error) {
	rerankedCandidates, err := s.rerankCandidates(ctx, query, candidates)
	if err != nil {
		return nil, err
	}
	recallLogger.Printf("reranked_candidates=%v", summarizeRerankedCandidates(rerankedCandidates))
	results := s.scoreCandidates(rerankedCandidates)
	recallLogger.Printf("final_scores=%v", summarizeRecallResults(results))
	return results, nil
}

func (s *Service) finalizeRecallResults(results []memory.RecallResult, input memory.RecallInput) []memory.RecallResult {
	// Softmax can surface weakly related candidates, so keep it behind the explicit flag.
	if enabelSoftmax {
		results = s.applySoftmaxScores(results, input.Temperature)
		recallLogger.Printf("final_probabilities=%v", summarizeRecallResults(results))
		return s.selectTopKByProbability(results, input.TopK)
	}

	if input.TopK < len(results) {
		return results[:input.TopK]
	}
	return results
}

func (s *Service) searchVectorCandidates(ctx context.Context, query string, topK int) ([]memory.RecallCandidate, error) {
	if s.embedder == nil {
		return nil, nil
	}
	vector, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, nil
	}
	return s.repo.VectorSearch(ctx, vector, topK*searchDepthMultiplier)
}

func (s *Service) searchFullTextCandidates(ctx context.Context, query string, topK int) ([]memory.RecallCandidate, error) {
	return s.repo.FullTextSearch(ctx, query, topK*searchDepthMultiplier)
}

func (s *Service) rrfCandidates(vectorCandidates, fullTextCandidates []memory.RecallCandidate, topK int) []memory.RecallCandidate {
	protectedVector := limitCandidates(vectorCandidates, topK*headSizeMultiplier)
	protectedFullText := limitCandidates(fullTextCandidates, topK*headSizeMultiplier)
	vectorRRFPool := sliceCandidates(vectorCandidates, topK*headSizeMultiplier, topK*rrfTailMultiplier)
	fullTextRRFPool := sliceCandidates(fullTextCandidates, topK*headSizeMultiplier, topK*rrfTailMultiplier)
	rankedIDs := scoring.RRF([][]string{extractIDs(vectorRRFPool), extractIDs(fullTextRRFPool)}, 10)
	if len(rankedIDs) > topK*rrfTopMultiplier {
		rankedIDs = rankedIDs[:topK*rrfTopMultiplier]
	}
	mergedCandidates := mergeCandidatesByID(vectorCandidates, fullTextCandidates)
	ordered := append([]memory.RecallCandidate(nil), protectedVector...)
	ordered = append(ordered, protectedFullText...)
	ordered = append(ordered, orderCandidatesByIDs(rankedIDs, mergedCandidates)...)
	return dedupeCandidates(ordered)
}

type rerankedCandidate struct {
	Candidate   memory.RecallCandidate
	RerankScore float64
}

func (s *Service) rerankCandidates(ctx context.Context, query string, candidates []memory.RecallCandidate) ([]rerankedCandidate, error) {
	if s.reranker == nil {
		return nil, errors.New("rerank provider is not configured")
	}

	documents := make([]string, 0, len(candidates))
	filteredCandidates := make([]memory.RecallCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Memory.Searchable() {
			continue
		}
		documents = append(documents, candidate.Memory.Content)
		filteredCandidates = append(filteredCandidates, candidate)
	}
	if len(filteredCandidates) == 0 {
		return nil, nil
	}

	rerankResults, err := s.reranker.Rerank(ctx, query, documents, len(documents))
	if err != nil {
		return nil, err
	}
	sort.Slice(rerankResults, func(i, j int) bool { return rerankResults[i].RelevanceScore > rerankResults[j].RelevanceScore })

	rerankedCandidates := make([]rerankedCandidate, 0, len(filteredCandidates))
	for _, rerankResult := range rerankResults {
		if rerankResult.Index < 0 || rerankResult.Index >= len(filteredCandidates) {
			continue
		}
		if rerankResult.RelevanceScore < rerankThreshold {
			continue
		}
		candidate := filteredCandidates[rerankResult.Index]
		rerankedCandidates = append(rerankedCandidates, rerankedCandidate{Candidate: candidate, RerankScore: rerankResult.RelevanceScore})
	}
	return rerankedCandidates, nil
}

func (s *Service) scoreCandidates(candidates []rerankedCandidate) []memory.RecallResult {
	inputs := make([]scoring.CandidateScoreInput, 0, len(candidates))
	for _, candidate := range candidates {
		inputs = append(inputs, scoring.CandidateScoreInput{Candidate: candidate.Candidate, RerankScore: candidate.RerankScore})
	}
	return scoring.Score(inputs)
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

func limitCandidates(candidates []memory.RecallCandidate, limit int) []memory.RecallCandidate {
	if limit <= 0 || len(candidates) <= limit {
		return append([]memory.RecallCandidate(nil), candidates...)
	}
	return append([]memory.RecallCandidate(nil), candidates[:limit]...)
}

func sliceCandidates(candidates []memory.RecallCandidate, offset, limit int) []memory.RecallCandidate {
	if offset >= len(candidates) || limit <= 0 {
		return nil
	}
	end := offset + limit
	if end > len(candidates) {
		end = len(candidates)
	}
	return append([]memory.RecallCandidate(nil), candidates[offset:end]...)
}

func dedupeCandidates(candidates []memory.RecallCandidate) []memory.RecallCandidate {
	seen := map[string]struct{}{}
	deduped := make([]memory.RecallCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := seen[candidate.Memory.ID]; ok {
			continue
		}
		seen[candidate.Memory.ID] = struct{}{}
		deduped = append(deduped, candidate)
	}
	return deduped
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
		out = append(out, contentSnippet(result.Memory.Content)+":"+formatFloat(result.Score))
	}
	return out
}

func summarizeRecallCandidates(candidates []memory.RecallCandidate) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out,
			contentSnippet(candidate.Memory.Content)+
				":distance="+formatOptionalFloat(candidate.VectorDistance)+
				":score="+formatOptionalFloat(candidate.FullTextScore)+
				"\n",
		)
	}
	return out
}

func summarizeRerankedCandidates(candidates []rerankedCandidate) []string {
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, contentSnippet(candidate.Candidate.Memory.Content)+":"+formatFloat(candidate.RerankScore))
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
