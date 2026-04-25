package scoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/memory"
)

func TestScoreRewardsFreshAndStoredMemories(t *testing.T) {
	now := time.Now().UTC()
	best := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{ID: "best", StoreCount: 3, CreatedAt: now, UpdatedAt: now}}, RerankScore: 0.8}
	worse := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{ID: "worse", StoreCount: 1, CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour)}}, RerankScore: 0.8}
	results := Score([]CandidateScoreInput{best, worse})

	require.Len(t, results, 2)
	require.Equal(t, "best", results[0].Memory.ID)
	require.Greater(t, results[0].Score, results[1].Score)
}

func TestScoreDoesNotBoostWeaklyRelevantCandidateTooMuch(t *testing.T) {
	now := time.Now().UTC()
	weakButFresh := CandidateScoreInput{Candidate: memory.RecallCandidate{Memory: memory.Memory{StoreCount: 10, UpdatedAt: now}}, RerankScore: 0.59}

	results := Score([]CandidateScoreInput{weakButFresh})

	require.Len(t, results, 1)
	require.Greater(t, results[0].Score, 0.59)
}
