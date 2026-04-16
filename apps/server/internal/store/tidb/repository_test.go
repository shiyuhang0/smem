package tidb

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"smem/apps/server/internal/domain/memory"
)

func TestRepositoryCRUDAndList(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
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

func TestScanRecallCandidateRowsPreservesDistanceAndFullTextScore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
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
