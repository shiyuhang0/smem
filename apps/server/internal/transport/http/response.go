package http

import "smem/apps/server/internal/domain/memory"

type memoryResponse struct {
	ID             string         `json:"id"`
	Content        string         `json:"content"`
	Type           memory.Type    `json:"type,omitempty"`
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
}

type acceptedResponse struct {
	Accepted bool             `json:"accepted"`
	Items    []memoryResponse `json:"items"`
}
