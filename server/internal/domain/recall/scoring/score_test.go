package scoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/memory"
)

func TestScoreRewardsFreshAndStoredMemories(t *testing.T) {
	now := time.Now().UTC()
	best := memory.RecallCandidate{
		Memory:         memory.Memory{StoreCount: 3, CreatedAt: now, UpdatedAt: now},
		VectorDistance: floatPtr(0.05),
	}
	worse := memory.RecallCandidate{
		Memory:         memory.Memory{StoreCount: 1, CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour)},
		VectorDistance: floatPtr(0.35),
	}

	require.Greater(t, Score(ScoreInput{Candidate: best, Now: now, MaxStoreCount: 3}), Score(ScoreInput{Candidate: worse, Now: now, MaxStoreCount: 3}))
}

func TestScoreDoesNotBoostWeaklyRelevantCandidateTooMuch(t *testing.T) {
	now := time.Now().UTC()
	weakButFresh := memory.RecallCandidate{
		Memory:         memory.Memory{StoreCount: 10, UpdatedAt: now},
		VectorDistance: floatPtr(0.9),
	}

	score := Score(ScoreInput{
		Candidate:        weakButFresh,
		Now:              now,
		MaxStoreCount:    10,
		MaxFullTextScore: 1,
	})

	require.InDelta(t, 0.06, score, 0.000001)
}

func floatPtr(value float64) *float64 {
	return &value
}
