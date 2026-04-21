package memory

import "time"

type Memory struct {
	ID             string
	Content        string
	Embedding      []float32
	ContentHash    string
	Type           Type
	Kind           string
	Kinds          []string
	Scope          Scope
	State          State
	Metadata       map[string]any
	AgentID        string
	SessionID      string
	Source         string
	Version        int
	StoreCount     int
	UseCount       int
	LastAccessedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func PrimaryKind(kinds []string) string {
	if len(kinds) == 0 {
		return ""
	}
	return kinds[0]
}

func (m Memory) Searchable() bool {
	return m.State == StateActive
}

type CreateInput struct {
	Content   string
	Mode      Mode
	Type      Type
	Kinds     []string
	Scope     Scope
	Metadata  map[string]any
	AgentID   string
	SessionID string
	Source    string
}

type UpdateInput struct {
	Content  *string
	Type     *Type
	Kinds    []string
	Scope    *Scope
	State    *State
	Metadata map[string]any
}

type ListInput struct {
	Page     int
	PageSize int
	Search   string
	Kind     string
	State    State
	Type     Type
}

type KindCount struct {
	Kind  string
	Count int64
}

type RecallInput struct {
	Content     string
	TopK        int
	Temperature float64
}

type RecallCandidate struct {
	Memory         Memory
	VectorDistance *float64
	FullTextScore  *float64
}

type RecallResult struct {
	Memory Memory
	Score  float64
	Reason string
}
