package ingestjob

type State string

const (
	StatePending   State = "pending"
	StateRunning   State = "running"
	StateSucceeded State = "succeeded"
	StateFailed    State = "failed"
)

type Mode string

const (
	ModeNormal Mode = "normal"
	ModeSmart  Mode = "smart"
)
