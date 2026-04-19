package ingestjob

import (
	"fmt"
	"time"

	"smem/apps/server/internal/domain/memory"
)

var (
	ErrNotFound = fmt.Errorf("ingest job not found")
	ErrConflict = fmt.Errorf("ingest job state conflict")
)

type Job struct {
	ID              string
	Content         string
	Type            memory.Type
	Kinds           []string
	Scope           memory.Scope
	Mode            Mode
	State           State
	Metadata        map[string]any
	AgentID         string
	SessionID       string
	Source          string
	ExecuteCount    int
	NextRunAt       *time.Time
	LockedAt        *time.Time
	WorkerID        string
	LastError       string
	ResultMemoryIDs []string
	ResultSummary   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
