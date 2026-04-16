package tidb

import (
	"crypto/tls"
	"fmt"
	"strings"

	gosqlmysql "github.com/go-sql-driver/mysql"

	"smem/apps/server/internal/config"
)

func PrepareDSN(cfg config.Config) (string, error) {
	if cfg.DBTLSServerName == "" || !strings.Contains(cfg.DBDSN, "tls=tidb") {
		return cfg.DBDSN, nil
	}
	if err := gosqlmysql.RegisterTLSConfig("tidb", &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: cfg.DBTLSServerName,
	}); err != nil && !strings.Contains(err.Error(), "already exists") {
		return "", fmt.Errorf("register db tls config: %w", err)
	}
	return cfg.DBDSN, nil
}
