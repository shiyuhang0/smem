package scoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/memory"
)

func TestScoreRewardsFreshFrequentlyUsedMemories(t *testing.T) {
	now := time.Now().UTC()
	best := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{ID: "best", StoreCount: 3, UseCount: 5, CreatedAt: now, UpdatedAt: now}}, RerankScore: 0.8}
	worse := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{ID: "worse", StoreCount: 1, UseCount: 0, CreatedAt: now.Add(-14 * 24 * time.Hour), UpdatedAt: now.Add(-14 * 24 * time.Hour)}}, RerankScore: 0.8}
	results := Score([]CandidateScoreInput{worse, best})

	require.Len(t, results, 2)
	require.Equal(t, "best", results[0].Memory.ID)
	require.Greater(t, results[0].Score, results[1].Score)
}

func TestScoreUseCountBreaksCloseTie(t *testing.T) {
	now := time.Now().UTC()
	highUse := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{ID: "high-use", StoreCount: 1, UseCount: 20, UpdatedAt: now.Add(-24 * time.Hour)}}, RerankScore: 0.8}
	lowUse := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{ID: "low-use", StoreCount: 1, UseCount: 1, UpdatedAt: now.Add(-24 * time.Hour)}}, RerankScore: 0.8}
	results := Score([]CandidateScoreInput{lowUse, highUse})

	require.Equal(t, "high-use", results[0].Memory.ID)
}

func TestStoreCountScoreUsesLogNormalization(t *testing.T) {
	linearLikeGap := storeCountScore(10, 100)
	smallValue := storeCountScore(1, 100)

	require.Greater(t, linearLikeGap, smallValue)
	require.Greater(t, linearLikeGap, 0.1)
	require.Less(t, linearLikeGap, 0.6)
}

func TestRecencyScoreUsesSevenDayHalfLife(t *testing.T) {
	now := time.Now().UTC()
	oneDay := recencyScore(now.Add(-24*time.Hour), now)
	sevenDays := recencyScore(now.Add(-7*24*time.Hour), now)
	fourteenDays := recencyScore(now.Add(-14*24*time.Hour), now)

	require.Greater(t, oneDay, sevenDays)
	require.InDelta(t, 0.5, sevenDays, 0.000001)
	require.InDelta(t, 0.25, fourteenDays, 0.000001)
}

func TestScoreKeepsRerankAsPrimarySignal(t *testing.T) {
	now := time.Now().UTC()
	highRerank := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{ID: "high-rerank", StoreCount: 1, UseCount: 0, UpdatedAt: now.Add(-30 * 24 * time.Hour)}}, RerankScore: 0.8}
	highBoost := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{ID: "high-boost", StoreCount: 100, UseCount: 100, UpdatedAt: now}}, RerankScore: 0.72}
	results := Score([]CandidateScoreInput{highBoost, highRerank})

	require.Equal(t, "high-rerank", results[0].Memory.ID)
	require.Greater(t, results[0].Score, results[1].Score)
}
