package tidb

import (
	"crypto/tls"
	"fmt"
	"strings"

	gosqlmysql "github.com/go-sql-driver/mysql"

	"smem/apps/server/internal/config"
)

func PrepareDSN(cfg config.Config) (string, error) {
	if cfg.DBTLSServerName != "" {
		if err := gosqlmysql.RegisterTLSConfig("tidb", &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: cfg.DBTLSServerName,
		}); err != nil && !strings.Contains(err.Error(), "already exists") {
			return "", fmt.Errorf("register db tls config: %w", err)
		}
	}

	dsn, err := gosqlmysql.ParseDSN(cfg.DBDSN)
	if err != nil {
		return "", fmt.Errorf("parse db dsn: %w", err)
	}
	dsn.ParseTime = true
	if cfg.DBTLSServerName != "" {
		dsn.TLSConfig = "tidb"
	}

	return dsn.FormatDSN(), nil
}
