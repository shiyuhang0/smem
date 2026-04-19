package ingest

import (
	"context"
	"time"

	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/observability"
)

var ingestLogger = observability.NewLogger("[ingest] ")

type Service struct {
	jobs ingestjob.Repository
	id   func() string
	now  func() time.Time
}

func NewService(jobs ingestjob.Repository, id func() string) *Service {
	return &Service{jobs: jobs, id: id, now: time.Now}
}

func (s *Service) Create(ctx context.Context, input memory.CreateInput) (ingestjob.Job, error) {
	if err := input.Validate(); err != nil {
		return ingestjob.Job{}, err
	}

	now := s.now().UTC()
	scope := input.Scope
	if scope == "" {
		scope = memory.ScopeUser
	}
	job := ingestjob.Job{
		ID:           s.id(),
		Content:      input.Content,
		Type:         input.Type,
		Kinds:        append([]string(nil), input.Kinds...),
		Scope:        scope,
		Mode:         jobModeFromMemory(input.Mode),
		State:        ingestjob.StatePending,
		Metadata:     input.Metadata,
		AgentID:      input.AgentID,
		SessionID:    input.SessionID,
		Source:       input.Source,
		ExecuteCount: 0,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	submitted, err := s.jobs.Submit(ctx, job)
	if err != nil {
		return ingestjob.Job{}, err
	}
	ingestLogger.Printf("job_submitted job_id=%s mode=%s", submitted.ID, submitted.Mode)
	return submitted, nil
}
