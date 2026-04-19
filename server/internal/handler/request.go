package handler

type createMemoryRequest struct {
	Content   string         `json:"content"`
	Mode      string         `json:"mode"`
	Type      string         `json:"type"`
	Kinds     []string       `json:"kinds"`
	Scope     string         `json:"scope"`
	Metadata  map[string]any `json:"metadata"`
	AgentID   string         `json:"agent_id"`
	SessionID string         `json:"session_id"`
	Source    string         `json:"source"`
}

type updateMemoryRequest struct {
	Content  *string        `json:"content"`
	Type     *string        `json:"type"`
	Kinds    []string       `json:"kinds"`
	Scope    *string        `json:"scope"`
	State    *string        `json:"state"`
	Metadata map[string]any `json:"metadata"`
}

type recallRequest struct {
	Content     string  `json:"content"`
	TopK        int     `json:"top_k"`
	Temperature float64 `json:"temperature"`
}
