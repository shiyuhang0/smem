package tidb

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"smem/apps/server/internal/domain/memory"
)

type Repository struct {
	db *gorm.DB
}

const memorySelectColumns = `
id,
content,
content_hash,
type,
kinds,
scope,
state,
metadata,
agent_id,
session_id,
source,
version,
store_count,
use_count,
last_accessed_at,
created_at,
updated_at
`

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

func (r *Repository) VectorSearch(ctx context.Context, queryVector []float32, limit int) ([]memory.RecallCandidate, error) {
	if limit <= 0 {
		limit = 10
	}
	if len(queryVector) == 0 {
		return nil, nil
	}
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT `+memorySelectColumns+`,
		       vec_cosine_distance(embedding, ?) AS distance
		FROM memories
		WHERE state = ? AND embedding IS NOT NULL
		ORDER BY distance
		LIMIT ?
	`, vectorLiteral(queryVector), string(memory.StateActive), limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRecallCandidateRows(r.db, rows)
}

func (r *Repository) FullTextSearch(ctx context.Context, query string, limit int) ([]memory.RecallCandidate, error) {
	if limit <= 0 {
		limit = 10
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	queryLiteral := fullTextQueryLiteral(query)
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT `+memorySelectColumns+`,
		       fts_match_word(`+queryLiteral+`, content) AS score
		FROM memories
		WHERE state = ? AND fts_match_word(`+queryLiteral+`, content)
		ORDER BY score DESC
		LIMIT ?
	`, string(memory.StateActive), limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRecallCandidateRows(r.db, rows)
}

func scanMemoryRows(db *gorm.DB, rows *sql.Rows) ([]memory.Memory, error) {
	out := make([]memory.Memory, 0)
	for rows.Next() {
		var model MemoryModel
		if err := db.ScanRows(rows, &model); err != nil {
			return nil, err
		}
		out = append(out, model.toDomain())
	}
	return out, rows.Err()
}

type recallCandidateRow struct {
	MemoryModel
	Distance *float64 `gorm:"column:distance"`
	Score    *float64 `gorm:"column:score"`
}

func scanRecallCandidateRows(db *gorm.DB, rows *sql.Rows) ([]memory.RecallCandidate, error) {
	out := make([]memory.RecallCandidate, 0)
	for rows.Next() {
		var row recallCandidateRow
		if err := db.ScanRows(rows, &row); err != nil {
			return nil, err
		}
		out = append(out, memory.RecallCandidate{
			Memory:         row.MemoryModel.toDomain(),
			VectorDistance: row.Distance,
			FullTextScore:  row.Score,
		})
	}
	return out, rows.Err()
}

func vectorLiteral(values []float32) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	return fmt.Sprintf("[%s]", strings.Join(parts, ","))
}

func fullTextQueryLiteral(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `'`, `''`)
	return "'" + value + "'"
}
