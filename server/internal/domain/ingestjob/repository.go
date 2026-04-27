package ingestjob

import (
	"context"
	"time"
)

type Repository interface {
	Submit(context.Context, Job) (Job, error)
	GetByID(context.Context, string) (Job, error)
	ClaimNext(context.Context, string, time.Time) (Job, error)
	ClaimBatchByAnchor(context.Context, Job, string, time.Time, int, time.Duration) ([]Job, error)
	MarkRetry(context.Context, Job, time.Time, string, time.Time) (Job, error)
	MarkFailed(context.Context, Job, string, time.Time) (Job, error)
	MarkSucceeded(context.Context, Job, []string, string, time.Time) (Job, error)
}
