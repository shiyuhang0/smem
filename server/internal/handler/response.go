package handler

import (
	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"
)

type memoryResponse struct {
	ID             string         `json:"id"`
	Content        string         `json:"content"`
	Type           memory.Type    `json:"type,omitempty"`
	Kind           string         `json:"kind,omitempty"`
	Kinds          []string       `json:"kinds,omitempty"`
	Scope          memory.Scope   `json:"scope"`
	State          memory.State   `json:"state"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	AgentID        string         `json:"agent_id,omitempty"`
	SessionID      string         `json:"session_id,omitempty"`
	Source         string         `json:"source,omitempty"`
	Version        int            `json:"version"`
	StoreCount     int            `json:"store_count"`
	UseCount       int            `json:"use_count"`
	LastAccessedAt *string        `json:"last_accessed_at,omitempty"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

type listMemoriesResponse struct {
	Items      []memoryResponse `json:"items"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	Total      int64            `json:"total"`
	TotalPages int              `json:"total_pages"`
	HasMore    bool             `json:"has_more"`
}

type kindCountResponse struct {
	Kind  string `json:"kind"`
	Count int64  `json:"count"`
}

type listKindsResponse struct {
	Items []kindCountResponse `json:"items"`
}

type ingestJobResponse struct {
	ID           string          `json:"id"`
	State        ingestjob.State `json:"state"`
	Mode         ingestjob.Mode  `json:"mode"`
	ExecuteCount int             `json:"execute_count"`
	LastError    string          `json:"last_error,omitempty"`
	NextRunAt    *string         `json:"next_run_at,omitempty"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}
