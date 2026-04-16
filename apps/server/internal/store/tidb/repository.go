package tidb

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"smem/apps/server/internal/domain/memory"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, item memory.Memory) (memory.Memory, error) {
	model := fromDomain(item)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return memory.Memory{}, err
	}
	return model.toDomain(), nil
}

func (r *Repository) Update(ctx context.Context, item memory.Memory) (memory.Memory, error) {
	model := fromDomain(item)
	result := r.db.WithContext(ctx).Model(&MemoryModel{}).Where("id = ?", item.ID).Updates(&model)
	if result.Error != nil {
		return memory.Memory{}, result.Error
	}
	if result.RowsAffected == 0 {
		return memory.Memory{}, memory.ErrNotFound
	}
	return r.GetByID(ctx, item.ID)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&MemoryModel{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return memory.ErrNotFound
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (memory.Memory, error) {
	var model MemoryModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return memory.Memory{}, memory.ErrNotFound
		}
		return memory.Memory{}, err
	}
	return model.toDomain(), nil
}

func (r *Repository) GetByContentHash(ctx context.Context, hash string) (memory.Memory, error) {
	var model MemoryModel
	err := r.db.WithContext(ctx).First(&model, "content_hash = ?", hash).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return memory.Memory{}, memory.ErrNotFound
		}
		return memory.Memory{}, err
	}
	return model.toDomain(), nil
}

func (r *Repository) List(ctx context.Context, input memory.ListInput) ([]memory.Memory, int64, error) {
	query := r.db.WithContext(ctx).Model(&MemoryModel{})
	if input.Search != "" {
		like := "%" + strings.TrimSpace(input.Search) + "%"
		query = query.Where("content LIKE ?", like)
	}
	if input.State != "" {
		query = query.Where("state = ?", string(input.State))
	}
	if input.Type != "" {
		query = query.Where("type = ?", string(input.Type))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (input.Page - 1) * input.PageSize
	if offset < 0 {
		offset = 0
	}
	var models []MemoryModel
	if err := query.Order("updated_at desc").Limit(input.PageSize).Offset(offset).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	out := make([]memory.Memory, 0, len(models))
	for _, model := range models {
		out = append(out, model.toDomain())
	}
	return out, total, nil
}

func (r *Repository) Search(ctx context.Context, query string, limit int) ([]memory.Memory, error) {
	if limit <= 0 {
		limit = 10
	}
	var models []MemoryModel
	like := "%" + strings.TrimSpace(query) + "%"
	err := r.db.WithContext(ctx).
		Where("state = ?", string(memory.StateActive)).
		Where("content LIKE ?", like).
		Order("updated_at desc").
		Limit(limit).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]memory.Memory, 0, len(models))
	for _, model := range models {
		out = append(out, model.toDomain())
	}
	return out, nil
}
