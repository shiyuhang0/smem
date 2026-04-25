package scoring

import (
	"fmt"
	"math"
	"sort"
	"time"

	"smem/apps/server/internal/domain/memory"
)

const (
	recencyWeight = 0.5
	storeWeight   = 0.5
	//useWeight          = 0.3
	boostWeight        = 0.1
	recencyHalfLifeDay = 7
)

type CandidateScoreInput struct {
	Candidate   memory.RecallCandidate
	RerankScore float64
}

// muti-dimension scoring based on recency, store count, use count, etc.
func Score(candidates []CandidateScoreInput) []memory.RecallResult {
	now := time.Now().UTC()
	maxStoreCount := maxStoreCount(candidates)
	results := make([]memory.RecallResult, 0, len(candidates))
	for _, candidate := range candidates {
		useTime := candidate.Candidate.Memory.UpdatedAt
		if useTime.IsZero() {
			useTime = candidate.Candidate.Memory.CreatedAt
		}
		recency := recencyScore(useTime, now)
		store := storeCountScore(candidate.Candidate.Memory.StoreCount, maxStoreCount)
		fmt.Printf("[score]%s: recency:%.2f, store:%.2f\n", candidate.Candidate.Memory.Content, recency, store)
		boost := recencyWeight*recency + storeWeight*store
		score := candidate.RerankScore + boostWeight*boost
		results = append(results, memory.RecallResult{Memory: candidate.Candidate.Memory, Score: score})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results
}

// 0.5 ^ (ageHours / 168)
func recencyScore(ts time.Time, now time.Time) float64 {
	if ts.IsZero() {
		return 0
	}
	ageHours := now.Sub(ts).Hours()
	if ageHours < 0 {
		ageHours = 0
	}
	halfLifeHours := float64(recencyHalfLifeDay * 24)
	return math.Exp(-math.Ln2 * ageHours / halfLifeHours)
}

func storeCountScore(storeCount, maxStoreCount int) float64 {
	return logNormalizedCount(storeCount, maxStoreCount)
}

func useCountScore(useCount, maxUseCount int) float64 {
	return logNormalizedCount(useCount, maxUseCount)
}

// 长尾友好的对数比例
func logNormalizedCount(value, maxValue int) float64 {
	if value <= 0 || maxValue <= 0 {
		return 0
	}
	// ln(1 + value) / ln(1 + maxValue)，保证在 value=0 时得分为 0，value=maxValue 时得分为 1，并且对长尾友好。
	return math.Log1p(float64(value)) / math.Log1p(float64(maxValue))
}

func maxStoreCount(candidates []CandidateScoreInput) int {
	maxValue := 0
	for _, candidate := range candidates {
		if candidate.Candidate.Memory.StoreCount > maxValue {
			maxValue = candidate.Candidate.Memory.StoreCount
		}
	}
	return maxValue
}

func maxUseCount(candidates []CandidateScoreInput) int {
	maxValue := 0
	for _, candidate := range candidates {
		if candidate.Candidate.Memory.UseCount > maxValue {
			maxValue = candidate.Candidate.Memory.UseCount
		}
	}
	return maxValue
}
