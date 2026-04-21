package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"gorm.io/gorm"
)

func ApplyMigrations(ctx context.Context, db *gorm.DB) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := applyMigrationFile(ctx, db, path); err != nil {
			return err
		}
	}
	return nil
}

func applyMigrationFile(ctx context.Context, db *gorm.DB, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration file %s: %w", path, err)
	}
	for _, stmt := range splitSQLStatements(string(data)) {
		if err := db.WithContext(ctx).Exec(stmt).Error; err != nil {
			if isIgnorableMigrationError(err) {
				continue
			}
			return fmt.Errorf("apply migration %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}

func migrationsDir() (string, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve migrations dir")
	}
	return filepath.Join(filepath.Dir(currentFile), "..", "..", "migrations"), nil
}

func splitSQLStatements(content string) []string {
	parts := strings.Split(content, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		out = append(out, stmt)
	}
	return out
}

func isIgnorableMigrationError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "duplicate key name") ||
		strings.Contains(msg, "duplicate column name")
}
