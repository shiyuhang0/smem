package tidb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"smem/apps/server/internal/config"
	"smem/apps/server/internal/domain/memory"
)

func TestTiDBCloudConnection(t *testing.T) {
	if os.Getenv("SMEM_INTEGRATION_TIDB") != "1" {
		t.Skip("set SMEM_INTEGRATION_TIDB=1 to enable")
	}

	cfg := config.Config{
		DBDSN:           os.Getenv("DB_DSN"),
		DBTLSServerName: os.Getenv("DB_TLS_SERVER_NAME"),
	}
	dsn, err := PrepareDSN(cfg)
	require.NoError(t, err)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.PingContext(context.Background()))
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS memories").Error)
	require.NoError(t, ApplyMigrations(context.Background(), db))

	repo := NewRepository(db)
	now := time.Now().UTC()
	require.NoError(t, db.Exec("DELETE FROM memories WHERE id IN ?", []string{"vec-1", "vec-2"}).Error)
	items := []memory.Memory{
		{
			ID:          "vec-1",
			Content:     "bluetooth earbuds with noise cancelling",
			Embedding:   testVector(0),
			ContentHash: memory.HashContent("bluetooth earbuds with noise cancelling"),
			State:       memory.StateActive,
			Scope:       memory.ScopeUser,
			Version:     1,
			StoreCount:  1,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          "vec-2",
			Content:     "kitchen recipe with tomatoes",
			Embedding:   testVector(1),
			ContentHash: memory.HashContent("kitchen recipe with tomatoes"),
			State:       memory.StateActive,
			Scope:       memory.ScopeUser,
			Version:     1,
			StoreCount:  1,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	for _, item := range items {
		_, err := repo.Create(context.Background(), item)
		require.NoError(t, err)
	}

	vectorResults, err := repo.VectorSearch(context.Background(), testVector(0), 2)
	require.NoError(t, err)
	require.NotEmpty(t, vectorResults)
	require.Equal(t, "vec-1", vectorResults[0].Memory.ID)
	require.NotNil(t, vectorResults[0].VectorDistance)

	fullTextResults, err := repo.FullTextSearch(context.Background(), "bluetooth", 2)
	require.NoError(t, err)
	require.NotEmpty(t, fullTextResults)
	require.Equal(t, "vec-1", fullTextResults[0].Memory.ID)
	require.NotNil(t, fullTextResults[0].FullTextScore)
}

func testVector(index int) []float32 {
	values := make([]float32, 1536)
	if index >= 0 && index < len(values) {
		values[index] = 1
	}
	return values
}
