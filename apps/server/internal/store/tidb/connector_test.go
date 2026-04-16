package tidb

import (
	"testing"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/config"
)

func TestPrepareDSNRegistersTLSWhenServerNameConfigured(t *testing.T) {
	input := config.Config{
		DBDSN:           "user:pass@tcp(gateway01.ap-southeast-1.prod.aws.tidbcloud.com:4000)/test?tls=tidb",
		DBTLSServerName: "gateway01.ap-southeast-1.prod.aws.tidbcloud.com",
	}

	dsn, err := PrepareDSN(input)
	require.NoError(t, err)
	require.Contains(t, dsn, "tls=tidb")
}

func TestPrepareDSNLeavesPlainDSNUnchanged(t *testing.T) {
	input := config.Config{
		DBDSN: "user:pass@tcp(localhost:4000)/test?parseTime=true",
	}

	dsn, err := PrepareDSN(input)
	require.NoError(t, err)
	require.Equal(t, input.DBDSN, dsn)
}
