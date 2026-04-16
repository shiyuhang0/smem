package tidb

import (
	"context"
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
