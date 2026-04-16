package tidb

import (
	"context"

	"gorm.io/gorm"
)

func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).AutoMigrate(&MemoryModel{})
}
