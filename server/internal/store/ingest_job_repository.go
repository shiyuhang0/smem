package store

import (
	"context"
	"time"

	"gorm.io/gorm"

	"smem/apps/server/internal/domain/ingestjob"
)

type IngestJobRepository struct {
	db *gorm.DB
}

func NewIngestJobRepository(db *gorm.DB) *IngestJobRepository {
	return &IngestJobRepository{db: db}
}

func (r *IngestJobRepository) Submit(ctx context.Context, job ingestjob.Job) (ingestjob.Job, error) {
	model := ingestJobFromDomain(job)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return ingestjob.Job{}, err
	}
	return model.toDomain(), nil
}

func (r *IngestJobRepository) GetByID(ctx context.Context, id string) (ingestjob.Job, error) {
	var model IngestJobModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ingestjob.Job{}, ingestjob.ErrNotFound
		}
		return ingestjob.Job{}, err
	}
	return model.toDomain(), nil
}

func (r *IngestJobRepository) ClaimNext(ctx context.Context, workerID string, now time.Time) (ingestjob.Job, error) {
	for range 5 {
		claimed, err := r.claimNextOnce(ctx, workerID, now.UTC())
		if err == nil {
			return claimed, nil
		}
		if err == ingestjob.ErrConflict {
			continue
		}
		return ingestjob.Job{}, err
	}
	return ingestjob.Job{}, ingestjob.ErrConflict
}

func (r *IngestJobRepository) ClaimBatchByAnchor(ctx context.Context, anchor ingestjob.Job, workerID string, now time.Time, limit int, window time.Duration) ([]ingestjob.Job, error) {
	if limit <= 1 {
		return []ingestjob.Job{anchor}, nil
	}

	now = now.UTC()
	windowEnd := anchor.CreatedAt.UTC().Add(window)
	query := r.db.WithContext(ctx).
		Where("state = ?", string(ingestjob.StatePending)).
		Where("mode = ?", string(anchor.Mode)).
		Where("source = ?", anchor.Source).
		Where("session_id = ?", anchor.SessionID).
		Where("next_run_at IS NULL OR next_run_at <= ?", now).
		Where("created_at >= ?", anchor.CreatedAt.UTC()).
		Where("created_at < ?", windowEnd).
		Order("created_at asc").
		Limit(limit - 1)

	var models []IngestJobModel
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	batch := []ingestjob.Job{anchor}
	for _, model := range models {
		claimed, err := r.claimPendingJob(ctx, model, workerID, now)
		if err != nil {
			if err == ingestjob.ErrConflict {
				continue
			}
			return batch, err
		}
		batch = append(batch, claimed)
	}
	return batch, nil
}

func (r *IngestJobRepository) MarkRetry(ctx context.Context, job ingestjob.Job, nextRunAt time.Time, lastError string, now time.Time) (ingestjob.Job, error) {
	return r.updateClaimedJob(ctx, job, map[string]any{
		"state":       string(ingestjob.StatePending),
		"next_run_at": nextRunAt.UTC(),
		"locked_at":   nil,
		"worker_id":   "",
		"last_error":  lastError,
		"updated_at":  now.UTC(),
	})
}

func (r *IngestJobRepository) MarkFailed(ctx context.Context, job ingestjob.Job, lastError string, now time.Time) (ingestjob.Job, error) {
	return r.updateClaimedJob(ctx, job, map[string]any{
		"state":       string(ingestjob.StateFailed),
		"next_run_at": nil,
		"locked_at":   nil,
		"worker_id":   "",
		"last_error":  lastError,
		"updated_at":  now.UTC(),
	})
}

func (r *IngestJobRepository) MarkSucceeded(ctx context.Context, job ingestjob.Job, resultMemoryIDs []string, resultSummary string, now time.Time) (ingestjob.Job, error) {
	return r.updateClaimedJob(ctx, job, map[string]any{
		"state":             string(ingestjob.StateSucceeded),
		"next_run_at":       nil,
		"locked_at":         nil,
		"worker_id":         "",
		"last_error":        "",
		"result_memory_ids": StringSlice(resultMemoryIDs),
		"result_summary":    resultSummary,
		"updated_at":        now.UTC(),
	})
}

func (r *IngestJobRepository) claimPendingJob(ctx context.Context, model IngestJobModel, workerID string, now time.Time) (ingestjob.Job, error) {
	lockedAt := now.UTC()
	result := r.db.WithContext(ctx).Model(&IngestJobModel{}).
		Where("id = ?", model.ID).
		Where("state = ?", string(ingestjob.StatePending)).
		Where("execute_count = ?", model.ExecuteCount).
		Where("next_run_at IS NULL OR next_run_at <= ?", now).
		Updates(map[string]any{
			"state":         string(ingestjob.StateRunning),
			"execute_count": model.ExecuteCount + 1,
			"worker_id":     workerID,
			"locked_at":     lockedAt,
			"updated_at":    lockedAt,
		})
	if result.Error != nil {
		return ingestjob.Job{}, result.Error
	}
	if result.RowsAffected == 0 {
		return ingestjob.Job{}, ingestjob.ErrConflict
	}

	model.State = string(ingestjob.StateRunning)
	model.ExecuteCount++
	model.WorkerID = workerID
	model.LockedAt = &lockedAt
	model.UpdatedAt = lockedAt
	return model.toDomain(), nil
}

func (r *IngestJobRepository) claimNextOnce(ctx context.Context, workerID string, now time.Time) (ingestjob.Job, error) {
	var claimed ingestjob.Job
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var models []IngestJobModel
		err := tx.
			Where("state = ?", string(ingestjob.StatePending)).
			Where("next_run_at IS NULL OR next_run_at <= ?", now).
			Order("created_at asc").
			Limit(1).
			Find(&models).Error
		if err != nil {
			return err
		}
		if len(models) == 0 {
			return ingestjob.ErrNotFound
		}
		var claimErr error
		claimed, claimErr = (&IngestJobRepository{db: tx}).claimPendingJob(ctx, models[0], workerID, now)
		if claimErr != nil {
			return claimErr
		}
		return nil
	})
	if err != nil {
		return ingestjob.Job{}, err
	}
	return claimed, nil
}

func (r *IngestJobRepository) updateClaimedJob(ctx context.Context, job ingestjob.Job, updates map[string]any) (ingestjob.Job, error) {
	result := r.db.WithContext(ctx).Model(&IngestJobModel{}).
		Where("id = ?", job.ID).
		Where("state = ?", string(ingestjob.StateRunning)).
		Where("execute_count = ?", job.ExecuteCount).
		Where("worker_id = ?", job.WorkerID).
		Updates(updates)
	if result.Error != nil {
		return ingestjob.Job{}, result.Error
	}
	if result.RowsAffected == 0 {
		return ingestjob.Job{}, ingestjob.ErrConflict
	}
	return r.GetByID(ctx, job.ID)
}
