package memory

import "context"

type Repository interface {
	Create(context.Context, Memory) (Memory, error)
	Update(context.Context, Memory) (Memory, error)
	Delete(context.Context, string) error
	GetByID(context.Context, string) (Memory, error)
	GetByContentHash(context.Context, string) (Memory, error)
	List(context.Context, ListInput) ([]Memory, int64, error)
	Search(context.Context, string, int) ([]Memory, error)
	VectorSearch(context.Context, []float32, int) ([]RecallCandidate, error)
	FullTextSearch(context.Context, string, int) ([]RecallCandidate, error)
}
