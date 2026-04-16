package recall

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"

	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/embedding"
	"smem/apps/server/internal/llm"
	"smem/apps/server/internal/observability"
	searchfusion "smem/apps/server/internal/search/fusion"
	searchrerank "smem/apps/server/internal/search/rerank"
)

var recallLogger = observability.NewLogger("[recall] ")

type Service struct {
	repo     memory.Repository
	embedder embedding.Provider
	llm      llm.Provider
}

func NewService(repo memory.Repository, embedder embedding.Provider, llmProvider llm.Provider) *Service {
	return &Service{repo: repo, embedder: embedder, llm: llmProvider}
}

func (s *Service) Recall(ctx context.Context, input memory.RecallInput) ([]memory.RecallResult, error) {
	if input.TopK == 0 {
		input.TopK = 5
	}
	if input.Temperature == 0 {
		input.Temperature = 1
	}
	rewritten := rewriteResult{Content: input.Content}
	if s.llm != nil {
		raw, err := s.llm.GenerateText(ctx, llm.NewRecallRewritePrompt(input.Content))
		if err == nil && raw != "" {
			parsed := rewriteResult{}
			if json.Unmarshal([]byte(raw), &parsed) == nil && strings.TrimSpace(parsed.Content) != "" {
				rewritten = parsed
			} else {
				rewritten.Content = raw
			}
		}
	}
	recallLogger.Printf("rewritten_content=%q", rewritten.Content)
	query := rewritten.Content
	queryVector := []float32(nil)
	if s.embedder != nil {
		if vector, err := s.embedder.Embed(ctx, query); err == nil {
			queryVector = vector
		}
	}
	vectorCandidates, _, err := s.repo.List(ctx, memory.ListInput{Page: 1, PageSize: 1000, State: memory.StateActive})
	if err != nil {
		return nil, err
	}
	vectorRanked := rankBySimilarityScores(query, queryVector, vectorCandidates)
	recallLogger.Printf("vector_search_results=%v", summarizeScoredCandidates(vectorRanked, input.TopK))
	fullTextCandidates, err := s.repo.Search(ctx, query, input.TopK*2)
	if err != nil {
		fullTextCandidates = nil
	}
	recallLogger.Printf("full_search_results=%v", summarizeMemoryCandidates(fullTextCandidates, input.TopK))
	vectorIDs := extractScoredIDs(vectorRanked)
	fullTextIDs := extractIDs(fullTextCandidates)
	rrfScores := computeRRFScores([][]string{vectorIDs, fullTextIDs}, 60)
	mergedIDs := searchfusion.RRF([][]string{vectorIDs, fullTextIDs}, 60)
	recallLogger.Printf("rrf_scores=%v", summarizeRRFScores(mergedIDs, rrfScores, input.TopK))
	byID := map[string]memory.Memory{}
	for _, item := range vectorCandidates {
		byID[item.ID] = item
	}
	for _, item := range fullTextCandidates {
		byID[item.ID] = item
	}
	results := make([]memory.RecallResult, 0, len(mergedIDs))
	rawScores := make([]float64, 0, len(mergedIDs))
	for _, id := range mergedIDs {
		item, ok := byID[id]
		if !ok || !item.Searchable() {
			continue
		}
		score := searchrerank.Score(item, memory.Type(rewritten.Type), rewritten.Kinds)
		results = append(results, memory.RecallResult{Memory: item, Score: score, Reason: "hybrid_recall"})
		rawScores = append(rawScores, score)
	}
	if len(results) == 0 {
		return nil, nil
	}
	recallLogger.Printf("final_scores=%v", summarizeRecallResults(results, input.TopK))
	probs := searchrerank.Softmax(rawScores, input.Temperature)
	for i := range results {
		results[i].Score = probs[i]
	}
	recallLogger.Printf("final_probabilities=%v", summarizeRecallResults(results, input.TopK))
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > input.TopK {
		results = results[:input.TopK]
	}
	return results, nil
}

type rewriteResult struct {
	Content string   `json:"content"`
	Type    string   `json:"type"`
	Kinds   []string `json:"kinds"`
}

type scoredCandidate struct {
	ID      string
	Score   float64
	Content string
}

func extractIDs(items []memory.Memory) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func extractScoredIDs(items []scoredCandidate) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func rankBySimilarityScores(query string, queryVector []float32, items []memory.Memory) []scoredCandidate {
	scoredItems := make([]scoredCandidate, 0, len(items))
	for _, item := range items {
		score := cosineLike(queryVector, item.Embedding)
		if score == 0 {
			score = textOverlapScore(query, item.Content)
		}
		scoredItems = append(scoredItems, scoredCandidate{ID: item.ID, Score: score, Content: contentSnippet(item.Content)})
	}
	sort.Slice(scoredItems, func(i, j int) bool { return scoredItems[i].Score > scoredItems[j].Score })
	return scoredItems
}

func cosineLike(left, right []float32) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	var dot, leftNorm, rightNorm float64
	for i := 0; i < limit; i++ {
		lv := float64(left[i])
		rv := float64(right[i])
		dot += lv * rv
		leftNorm += lv * lv
		rightNorm += rv * rv
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func textOverlapScore(query, content string) float64 {
	score := 0.0
	for _, token := range []rune(query) {
		for _, contentRune := range []rune(content) {
			if token == contentRune {
				score++
			}
		}
	}
	return score
}

func computeRRFScores(rankings [][]string, k float64) map[string]float64 {
	scores := map[string]float64{}
	for _, ranking := range rankings {
		for i, id := range ranking {
			scores[id] += 1.0 / (k + float64(i+1))
		}
	}
	return scores
}

func summarizeScoredCandidates(items []scoredCandidate, limit int) []string {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID+":"+formatFloat(item.Score))
	}
	return out
}

func summarizeMemoryCandidates(items []memory.Memory, limit int) []string {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID+":"+contentSnippet(item.Content))
	}
	return out
}

func summarizeRRFScores(ids []string, scores map[string]float64, limit int) []string {
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id+":"+formatFloat(scores[id]))
	}
	return out
}

func summarizeRecallResults(results []memory.RecallResult, limit int) []string {
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.Memory.ID+":"+formatFloat(result.Score))
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
