package memory

type Type string

const (
	TypeUnknown    Type = ""
	TypeFact       Type = "fact"
	TypeEpisodic   Type = "episodic"
	TypeProcedural Type = "procedural"
)

type Scope string

const (
	ScopeUser     Scope = "user"
	ScopeAgent    Scope = "agent"
	ScopeExternal Scope = "external"
)

type State string

const (
	StateCreating State = "creating"
	StateActive   State = "active"
	StateArchived State = "archived"
)

type Mode string

const (
	ModeNormal Mode = "normal"
	ModeSmart  Mode = "smart"
)
