package rerank

import (
	"time"

	"smem/apps/server/internal/domain/memory"
)

type ScoreInput struct {
	Candidate        memory.RecallCandidate
	Now              time.Time
	MaxStoreCount    int
	MaxFullTextScore float64
}

func Score(input ScoreInput) float64 {
	vector := vectorSimilarity(input.Candidate.VectorDistance)
	fulltext := fullTextScore(input.Candidate.FullTextScore, input.MaxFullTextScore)
	recency := recencyScore(input.Candidate.Memory.UpdatedAt, input.Now)
	store := storeCountScore(input.Candidate.Memory.StoreCount, input.MaxStoreCount)

	relevance := 0.6*vector + 0.4*fulltext
	if relevance < 0.2 {
		return relevance
	}

	boost := 0.7*recency + 0.3*store
	return relevance + 0.1*boost
}

func vectorSimilarity(distance *float64) float64 {
	if distance == nil {
		return 0
	}
	value := 1 - *distance
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
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

func fullTextScore(score *float64, maxScore float64) float64 {
	if score == nil || maxScore <= 0 {
		return 0
	}
	value := *score / maxScore
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
