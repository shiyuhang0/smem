package tidb

import (
	"testing"

	gosqlmysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/config"
)

func TestPrepareDSNRegistersTLSWhenServerNameConfigured(t *testing.T) {
	input := config.Config{
		DBDSN:           "user:pass@tcp(gateway01.ap-southeast-1.prod.aws.tidbcloud.com:4000)/test",
		DBTLSServerName: "gateway01.ap-southeast-1.prod.aws.tidbcloud.com",
	}

	dsn, err := PrepareDSN(input)
	require.NoError(t, err)
	parsed, err := gosqlmysql.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "tidb", parsed.TLSConfig)
	require.True(t, parsed.ParseTime)
}

func TestPrepareDSNAddsParseTimeToPlainDSN(t *testing.T) {
	input := config.Config{
		DBDSN: "user:pass@tcp(localhost:4000)/test",
	}

	dsn, err := PrepareDSN(input)
	require.NoError(t, err)
	parsed, err := gosqlmysql.ParseDSN(dsn)
	require.NoError(t, err)
	require.True(t, parsed.ParseTime)
	require.Empty(t, parsed.TLSConfig)
}
