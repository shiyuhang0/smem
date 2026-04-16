package rerank

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/domain/memory"
)

func TestScoreRewardsFreshAndStoredMemories(t *testing.T) {
	now := time.Now().UTC()
	best := memory.Memory{Type: memory.TypeFact, Kinds: []string{"note"}, StoreCount: 3, UpdatedAt: now}
	worse := memory.Memory{Type: memory.TypeFact, Kinds: []string{"note"}, StoreCount: 1, UpdatedAt: now.Add(-24 * time.Hour)}

	require.Greater(t, Score(best, memory.TypeFact, []string{"note"}), Score(worse, memory.TypeFact, []string{"note"}))
}
