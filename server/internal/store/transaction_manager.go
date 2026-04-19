package store

import (
	"context"

	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"

	"gorm.io/gorm"
)

type TransactionManager struct {
	db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) *TransactionManager {
	return &TransactionManager{db: db}
}

func (m *TransactionManager) Run(ctx context.Context, fn func(memory.Repository, ingestjob.Repository) error) error {
	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(NewRepository(tx), NewIngestJobRepository(tx))
	})
}
