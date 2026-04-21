package store

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"
)

func TestRepositoryCRUDAndList(t *testing.T) {
	db, err := openTestDB(t)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MemoryModel{}))

	repo := NewRepository(db)
	now := time.Unix(100, 0).UTC()
	item := memory.Memory{
		ID:          "m1",
		Content:     "remember this",
		ContentHash: memory.HashContent("remember this"),
		State:       memory.StateActive,
		Scope:       memory.ScopeUser,
		Kinds:       []string{"note"},
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := repo.Create(context.Background(), item)
	require.NoError(t, err)
	require.Equal(t, "m1", created.ID)

	loaded, err := repo.GetByID(context.Background(), "m1")
	require.NoError(t, err)
	require.Equal(t, "remember this", loaded.Content)
	require.Equal(t, "note", loaded.Kind)

	loaded.Content = "remember this better"
	updated, err := repo.Update(context.Background(), loaded)
	require.NoError(t, err)
	require.Equal(t, "remember this better", updated.Content)

	items, total, err := repo.List(context.Background(), memory.ListInput{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.EqualValues(t, 1, total)

	require.NoError(t, repo.Delete(context.Background(), "m1"))
	_, err = repo.GetByID(context.Background(), "m1")
	require.ErrorIs(t, err, memory.ErrNotFound)
}

func TestRepositoryUpsertByContentHashIncrementsStoreCount(t *testing.T) {
	db, err := openTestDB(t)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MemoryModel{}))

	repo := NewRepository(db)
	now := time.Unix(120, 0).UTC()
	first := memory.Memory{
		ID:          "m1",
		Content:     "remember this",
		ContentHash: memory.HashContent("remember this"),
		Embedding:   []float32{0.1, 0.2},
		State:       memory.StateActive,
		Scope:       memory.ScopeUser,
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	second := memory.Memory{
		ID:          "m2",
		Content:     "remember this",
		ContentHash: memory.HashContent("remember this"),
		Embedding:   []float32{0.3, 0.4},
		State:       memory.StateActive,
		Scope:       memory.ScopeUser,
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now.Add(time.Second),
		UpdatedAt:   now.Add(time.Second),
	}

	created, err := repo.UpsertByContentHash(context.Background(), first)
	require.NoError(t, err)
	require.Equal(t, "m1", created.ID)
	require.Equal(t, 1, created.StoreCount)

	upserted, err := repo.UpsertByContentHash(context.Background(), second)
	require.NoError(t, err)
	require.Equal(t, "m1", upserted.ID)
	require.Equal(t, 2, upserted.StoreCount)
	require.Equal(t, []float32{0.3, 0.4}, upserted.Embedding)
}

func TestScanRecallCandidateRowsPreservesDistanceAndFullTextScore(t *testing.T) {
	db, err := openTestDB(t)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MemoryModel{}))

	now := time.Unix(100, 0).UTC()
	require.NoError(t, db.Create(&MemoryModel{
		ID:          "m1",
		Content:     "remember this",
		ContentHash: "hash-1",
		Scope:       string(memory.ScopeUser),
		State:       string(memory.StateActive),
		Kinds:       StringSlice{},
		Metadata:    JSONMap{},
		Version:     1,
		StoreCount:  2,
		UseCount:    0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error)

	rows, err := db.Raw(`
		SELECT
			id,
			content,
			content_hash,
			type,
			kind,
			kinds,
			scope,
			state,
			metadata,
			agent_id,
			session_id,
			source,
			version,
			store_count,
			use_count,
			last_accessed_at,
			created_at,
			updated_at,
			0.12 AS distance,
			0.91 AS score
		FROM memories
		WHERE id = 'm1'
	`).Rows()
	require.NoError(t, err)
	defer rows.Close()

	candidates, err := scanRecallCandidateRows(db, rows)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].VectorDistance)
	require.NotNil(t, candidates[0].FullTextScore)
	require.True(t, math.Abs(*candidates[0].VectorDistance-0.12) < 0.000001)
	require.True(t, math.Abs(*candidates[0].FullTextScore-0.91) < 0.000001)
	require.Equal(t, "m1", candidates[0].Memory.ID)
}

func TestFullTextQueryLiteralEscapesSpecialCharacters(t *testing.T) {
	require.Equal(t, "'bluetooth'", fullTextQueryLiteral("bluetooth"))
	require.Equal(t, "'O''Reilly \\\\ guide'", fullTextQueryLiteral("O'Reilly \\ guide"))
}

func TestRepositoryUpsertByContentHashDerivesKindFromKinds(t *testing.T) {
	db, err := openTestDB(t)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MemoryModel{}))

	repo := NewRepository(db)
	now := time.Unix(120, 0).UTC()

	stored, err := repo.UpsertByContentHash(context.Background(), memory.Memory{
		ID:          "m1",
		Content:     "remember this",
		ContentHash: memory.HashContent("remember this"),
		State:       memory.StateActive,
		Scope:       memory.ScopeUser,
		Kinds:       []string{"preference", "note"},
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	require.NoError(t, err)
	require.Equal(t, "preference", stored.Kind)

	loaded, err := repo.GetByID(context.Background(), "m1")
	require.NoError(t, err)
	require.Equal(t, "preference", loaded.Kind)
}

func openTestDB(t *testing.T) (*gorm.DB, error) {
	t.Helper()
	return gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
}

func TestIngestJobRepositorySubmitClaimRetryAndSucceed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&IngestJobModel{}))

	repo := NewIngestJobRepository(db)
	now := time.Unix(200, 0).UTC()
	job := ingestjob.Job{
		ID:           "job-1",
		Content:      "remember this",
		Mode:         ingestjob.ModeNormal,
		Scope:        memory.ScopeUser,
		State:        ingestjob.StatePending,
		ExecuteCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	submitted, err := repo.Submit(context.Background(), job)
	require.NoError(t, err)
	require.Equal(t, ingestjob.StatePending, submitted.State)

	claimed, err := repo.ClaimNext(context.Background(), "worker-1", now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, "job-1", claimed.ID)
	require.Equal(t, ingestjob.StateRunning, claimed.State)
	require.Equal(t, 1, claimed.ExecuteCount)
	require.Equal(t, "worker-1", claimed.WorkerID)
	require.NotNil(t, claimed.LockedAt)

	requeued, err := repo.MarkRetry(context.Background(), claimed, now.Add(2*time.Second), "embedding failed", now.Add(3*time.Second))
	require.NoError(t, err)
	require.Equal(t, ingestjob.StatePending, requeued.State)
	require.Equal(t, "embedding failed", requeued.LastError)
	require.NotNil(t, requeued.NextRunAt)
	require.Nil(t, requeued.LockedAt)

	claimedAgain, err := repo.ClaimNext(context.Background(), "worker-2", now.Add(4*time.Second))
	require.NoError(t, err)
	require.Equal(t, 2, claimedAgain.ExecuteCount)

	succeeded, err := repo.MarkSucceeded(context.Background(), claimedAgain, []string{"mem-1", "mem-2"}, "created=1 updated=1", now.Add(5*time.Second))
	require.NoError(t, err)
	require.Equal(t, ingestjob.StateSucceeded, succeeded.State)
	require.Equal(t, []string{"mem-1", "mem-2"}, succeeded.ResultMemoryIDs)
	require.Equal(t, "created=1 updated=1", succeeded.ResultSummary)
	require.Nil(t, succeeded.LockedAt)
}

func TestIngestJobRepositoryMarksTerminalFailureAfterFifthAttempt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&IngestJobModel{}))

	repo := NewIngestJobRepository(db)
	now := time.Unix(400, 0).UTC()
	job := ingestjob.Job{
		ID:           "job-5",
		Content:      "remember this",
		Mode:         ingestjob.ModeSmart,
		Scope:        memory.ScopeUser,
		State:        ingestjob.StateRunning,
		ExecuteCount: 5,
		WorkerID:     "worker-1",
		LockedAt:     ptrTime(now),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	_, err = repo.Submit(context.Background(), job)
	require.NoError(t, err)

	failed, err := repo.MarkFailed(context.Background(), job, "llm protocol error", now.Add(time.Second))
	require.NoError(t, err)
	require.Equal(t, ingestjob.StateFailed, failed.State)
	require.Equal(t, "llm protocol error", failed.LastError)
	require.Nil(t, failed.LockedAt)
}

func TestIngestJobRepositoryRejectsStaleTerminalUpdate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&IngestJobModel{}))

	repo := NewIngestJobRepository(db)
	now := time.Unix(500, 0).UTC()
	job := ingestjob.Job{
		ID:        "job-stale",
		Content:   "remember this",
		Mode:      ingestjob.ModeNormal,
		Scope:     memory.ScopeUser,
		State:     ingestjob.StatePending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err = repo.Submit(context.Background(), job)
	require.NoError(t, err)

	claimed, err := repo.ClaimNext(context.Background(), "worker-1", now.Add(time.Second))
	require.NoError(t, err)

	_, err = repo.MarkSucceeded(context.Background(), claimed, []string{"mem-1"}, "created=1 updated=0", now.Add(2*time.Second))
	require.NoError(t, err)

	_, err = repo.MarkRetry(context.Background(), claimed, now.Add(3*time.Second), "should be stale", now.Add(4*time.Second))
	require.ErrorIs(t, err, ingestjob.ErrConflict)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
