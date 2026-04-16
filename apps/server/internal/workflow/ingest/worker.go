package ingest

import (
	"context"

	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/embedding"
)

type EmbeddingWorker struct {
	repo     memory.Repository
	embedder embedding.Provider
	maxDim   int
}

func NewEmbeddingWorker(repo memory.Repository, embedder embedding.Provider, maxDim int) *EmbeddingWorker {
	return &EmbeddingWorker{repo: repo, embedder: embedder, maxDim: maxDim}
}

func (w *EmbeddingWorker) Queue(ctx context.Context, item memory.Memory) error {
	vector, err := w.embedder.Embed(ctx, item.Content)
	if err != nil {
		return err
	}
	item.Embedding = vector
	item.State = memory.StateActive
	_, err = w.repo.Update(ctx, item)
	return err
}
