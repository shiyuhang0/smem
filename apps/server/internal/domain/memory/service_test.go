package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServiceCreateDeduplicatesByContentHash(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, func() string { return "memory-1" })
	svc.now = func() time.Time { return time.Unix(100, 0).UTC() }

	first, err := svc.Create(context.Background(), CreateInput{Content: "remember this", Mode: ModeNormal})
	require.NoError(t, err)
	second, err := svc.Create(context.Background(), CreateInput{Content: "remember this", Mode: ModeNormal})
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, 2, second.StoreCount)
}

type fakeRepository struct {
	items map[string]Memory
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{items: map[string]Memory{}}
}

func (r *fakeRepository) Create(_ context.Context, m Memory) (Memory, error) {
	r.items[m.ID] = m
	return m, nil
}

func (r *fakeRepository) Update(_ context.Context, m Memory) (Memory, error) {
	r.items[m.ID] = m
	return m, nil
}

func (r *fakeRepository) Delete(_ context.Context, id string) error {
	delete(r.items, id)
	return nil
}

func (r *fakeRepository) GetByID(_ context.Context, id string) (Memory, error) {
	m, ok := r.items[id]
	if !ok {
		return Memory{}, ErrNotFound
	}
	return m, nil
}

func (r *fakeRepository) GetByContentHash(_ context.Context, hash string) (Memory, error) {
	for _, item := range r.items {
		if item.ContentHash == hash {
			return item, nil
		}
	}
	return Memory{}, ErrNotFound
}

func (r *fakeRepository) List(_ context.Context, _ ListInput) ([]Memory, int64, error) {
	out := make([]Memory, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, int64(len(out)), nil
}

func (r *fakeRepository) Search(_ context.Context, _ string, _ int) ([]Memory, error) {
	return nil, nil
}
