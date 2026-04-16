package recall

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strings"

	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/embedding"
	"smem/apps/server/internal/llm"
	searchfusion "smem/apps/server/internal/search/fusion"
	searchrerank "smem/apps/server/internal/search/rerank"
)

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
	fullTextCandidates, err := s.repo.Search(ctx, query, input.TopK*2)
	if err != nil {
		fullTextCandidates = nil
	}
	vectorIDs := rankBySimilarity(query, queryVector, vectorCandidates)
	fullTextIDs := extractIDs(fullTextCandidates)
	mergedIDs := searchfusion.RRF([][]string{vectorIDs, fullTextIDs}, 60)
	byID := map[string]memory.Memory{}
	for _, item := range vectorCandidates {
		byID[item.ID] = item
	}
	for _, item := range fullTextCandidates {
		byID[item.ID] = item
	}
	results := make([]memory.RecallResult, 0, len(mergedIDs))
	baseScores := make([]float64, 0, len(mergedIDs))
	for _, id := range mergedIDs {
		item, ok := byID[id]
		if !ok || !item.Searchable() {
			continue
		}
		score := searchrerank.Score(item, memory.Type(rewritten.Type), rewritten.Kinds)
		results = append(results, memory.RecallResult{Memory: item, Score: score, Reason: "hybrid_recall"})
		baseScores = append(baseScores, score)
	}
	if len(results) == 0 {
		return nil, nil
	}
	probs := searchrerank.Softmax(baseScores, input.Temperature)
	for i := range results {
		results[i].Score = probs[i]
	}
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

func extractIDs(items []memory.Memory) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func rankBySimilarity(query string, queryVector []float32, items []memory.Memory) []string {
	type scored struct {
		id    string
		score float64
	}
	scoredItems := make([]scored, 0, len(items))
	for _, item := range items {
		score := cosineLike(queryVector, item.Embedding)
		if score == 0 {
			score = textOverlapScore(query, item.Content)
		}
		scoredItems = append(scoredItems, scored{id: item.ID, score: score})
	}
	sort.Slice(scoredItems, func(i, j int) bool { return scoredItems[i].score > scoredItems[j].score })
	ids := make([]string, 0, len(scoredItems))
	for _, item := range scoredItems {
		ids = append(ids, item.id)
	}
	return ids
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
