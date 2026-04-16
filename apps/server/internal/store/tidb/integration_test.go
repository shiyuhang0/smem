package tidb

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"smem/apps/server/internal/config"
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
	require.NoError(t, AutoMigrate(context.Background(), db))
}
