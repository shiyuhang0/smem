package scoring

import (
	"sort"
	"time"

	"smem/apps/server/internal/domain/memory"
)

const (
	recencyWeight = 0.7
	storeWeight   = 0.3
	boostWeight   = 0.1
)

type CandidateScoreInput struct {
	Candidate   memory.RecallCandidate
	RerankScore float64
}

func Score(candidates []CandidateScoreInput) []memory.RecallResult {
	now := time.Now().UTC()
	maxStoreCount := maxStoreCount(candidates)
	results := make([]memory.RecallResult, 0, len(candidates))
	for _, candidate := range candidates {
		recency := recencyScore(candidate.Candidate.Memory.UpdatedAt, now)
		store := storeCountScore(candidate.Candidate.Memory.StoreCount, maxStoreCount)
		boost := recencyWeight*recency + storeWeight*store
		score := candidate.RerankScore + boostWeight*boost
		results = append(results, memory.RecallResult{Memory: candidate.Candidate.Memory, Score: score})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results
}

func recencyScore(ts time.Time, now time.Time) float64 {
	if ts.IsZero() {
		return 0
	}
	ageHours := now.Sub(ts).Hours()
	if ageHours < 0 {
		ageHours = 0
	}
	return 1.0 / (1.0 + ageHours/24.0)
}

func storeCountScore(storeCount, maxStoreCount int) float64 {
	if maxStoreCount <= 0 {
		return 0
	}
	return float64(storeCount) / float64(maxStoreCount)
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
