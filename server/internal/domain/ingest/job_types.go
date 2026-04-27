package ingest

import (
	"time"

	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"
)

const (
	maxExtractedMemories  = 5
	maxRecallPerCandidate = 3
	maxJobAttempts        = 5
	defaultPollInterval   = 5 * time.Second

	batchWindow  = time.Minute
	maxBatchJobs = 10
)

var allowedKinds = map[string]struct{}{
	"skill":      {},
	"task":       {},
	"lesson":     {},
	"workflow":   {},
	"preference": {},
	"profile":    {},
	"note":       {},
	"decision":   {},
}

type extractedMemory struct {
	Content string
	Type    memory.Type
	Kinds   []string
}

type candidateMemory struct {
	ID        string
	Content   string
	Type      memory.Type
	Kinds     []string
	Scope     memory.Scope
	Metadata  map[string]any
	AgentID   string
	SessionID string
	Source    string
}

type actionMemory struct {
	Content string `json:"content"`
}

type fusionAction struct {
	Target              string        `json:"target"`
	ID                  string        `json:"id"`
	Action              string        `json:"action"`
	Memory              *actionMemory `json:"memory,omitempty"`
	AbsorbedByMemoryIDs []string      `json:"absorbed_by_memory_ids,omitempty"`
}

type memoryWriteSet struct {
	creates         []memory.Memory
	updates         []memory.Memory
	deletes         []string
	resultMemoryIDs []string
	resultSummary   string
}

func jobModeFromMemory(mode memory.Mode) ingestjob.Mode {
	if mode == memory.ModeSmart {
		return ingestjob.ModeSmart
	}
	return ingestjob.ModeNormal
}
