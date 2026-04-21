package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/ai/llm"
	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"
)

func TestServiceCreateSubmitsPendingJob(t *testing.T) {
	jobs := newJobRepo()
	svc := NewService(jobs, func() string { return "job-1" })
	svc.now = func() time.Time { return time.Unix(100, 0).UTC() }

	job, err := svc.Create(context.Background(), memory.CreateInput{
		Content: "remember this",
		Mode:    memory.ModeNormal,
	})
	require.NoError(t, err)
	require.Equal(t, "job-1", job.ID)
	require.Equal(t, ingestjob.StatePending, job.State)
	require.Equal(t, ingestjob.ModeNormal, job.Mode)
}

func TestParseExtractionPayloadTruncatesAndFiltersKinds(t *testing.T) {
	raw := `{"memories":[
		{"content":"one","type":"fact","kinds":["preference","invalid"]},
		{"content":"two","type":"episodic","kinds":[]},
		{"content":"three","type":"procedural","kinds":["workflow"]},
		{"content":"four","type":"","kinds":["note"]},
		{"content":"five","type":"fact","kinds":["task"]},
		{"content":"six","type":"fact","kinds":["skill"]}
	]}`

	items, err := parseExtractionPayload(raw)
	require.NoError(t, err)
	require.Len(t, items, 5)
	require.Equal(t, []string{"preference"}, items[0].Kinds)
	require.Equal(t, "five", items[4].Content)
}

func TestParseFusionPayloadRejectsInvalidProtocol(t *testing.T) {
	_, err := parseFusionPayload(`{"actions":[{"target":"candidate","id":"c1","action":"create"}]}`,
		[]candidateMemory{{ID: "c1", Content: "one"}},
		nil,
	)
	require.Error(t, err)
}

func TestJobWorkerRunOnceNormalModeCreatesMemoryAndMarksSuccess(t *testing.T) {
	memories := newMemoryRepo()
	jobs := newJobRepo()
	tx := &fakeTxManager{memories: memories, jobs: jobs}
	embedder := fakeEmbedder{vectors: map[string][]float32{"remember this": {0.1, 0.2}}}
	worker := NewJobWorker(jobs, tx, nil, nil, embedder, func() string { return "mem-1" }, "worker-1")
	worker.now = func() time.Time { return time.Unix(200, 0).UTC() }

	_, err := jobs.Submit(context.Background(), ingestjob.Job{
		ID:        "job-1",
		Content:   "remember this",
		Mode:      ingestjob.ModeNormal,
		Scope:     memory.ScopeUser,
		State:     ingestjob.StatePending,
		CreatedAt: time.Unix(100, 0).UTC(),
		UpdatedAt: time.Unix(100, 0).UTC(),
	})
	require.NoError(t, err)

	err = worker.RunOnce(context.Background())
	require.NoError(t, err)

	stored, err := memories.GetByID(context.Background(), "mem-1")
	require.NoError(t, err)
	require.Equal(t, memory.StateActive, stored.State)
	require.Equal(t, []float32{0.1, 0.2}, stored.Embedding)
	require.Empty(t, stored.Kind)

	job, err := jobs.GetByID(context.Background(), "job-1")
	require.NoError(t, err)
	require.Equal(t, ingestjob.StateSucceeded, job.State)
	require.Equal(t, []string{"mem-1"}, job.ResultMemoryIDs)
}

func TestJobWorkerRunOnceSmartModeUpdatesExistingMemory(t *testing.T) {
	memories := newMemoryRepo()
	existing := memory.Memory{
		ID:          "existing-1",
		Content:     "user prefers vim",
		ContentHash: memory.HashContent("user prefers vim"),
		Type:        memory.TypeFact,
		Kinds:       []string{"preference"},
		Scope:       memory.ScopeUser,
		State:       memory.StateActive,
		Version:     1,
		StoreCount:  1,
		CreatedAt:   time.Unix(10, 0).UTC(),
		UpdatedAt:   time.Unix(10, 0).UTC(),
	}
	_, _ = memories.Create(context.Background(), existing)
	jobs := newJobRepo()
	tx := &fakeTxManager{memories: memories, jobs: jobs}
	recall := fakeRecallService{
		results: map[string][]memory.RecallResult{
			"user prefers neovim": {{Memory: existing, Score: 1, Reason: "test"}},
		},
	}
	embedder := fakeEmbedder{vectors: map[string][]float32{"user prefers neovim": {0.3, 0.4}}}
	llmProvider := &sequenceLLM{responses: []fakeResponse{
		{text: `{"memories":[{"content":"user prefers neovim","type":"fact","kinds":["preference"]}]}`},
		{text: `{"actions":[{"target":"candidate","id":"c1","action":"ignore","absorbed_by_memory_ids":["existing-1"]},{"target":"memory","id":"existing-1","action":"update","memory":{"content":"user prefers neovim"}}]}`},
	}}
	worker := NewJobWorker(jobs, tx, recall, llmProvider, embedder, func() string { return "mem-2" }, "worker-1")
	worker.now = func() time.Time { return time.Unix(300, 0).UTC() }

	_, err := jobs.Submit(context.Background(), ingestjob.Job{
		ID:        "job-2",
		Content:   "I prefer neovim",
		Mode:      ingestjob.ModeSmart,
		Scope:     memory.ScopeUser,
		State:     ingestjob.StatePending,
		CreatedAt: time.Unix(100, 0).UTC(),
		UpdatedAt: time.Unix(100, 0).UTC(),
	})
	require.NoError(t, err)

	err = worker.RunOnce(context.Background())
	require.NoError(t, err)

	updated, err := memories.GetByID(context.Background(), "existing-1")
	require.NoError(t, err)
	require.Equal(t, "user prefers neovim", updated.Content)
	require.Equal(t, "preference", updated.Kind)
	require.Equal(t, 2, updated.Version)
	require.Equal(t, 2, updated.StoreCount)
	require.Equal(t, []float32{0.3, 0.4}, updated.Embedding)
}

func TestJobWorkerRunOnceRetriesWhenEmbeddingFails(t *testing.T) {
	memories := newMemoryRepo()
	jobs := newJobRepo()
	tx := &fakeTxManager{memories: memories, jobs: jobs}
	worker := NewJobWorker(
		jobs,
		tx,
		nil,
		nil,
		fakeEmbedder{err: errors.New("boom")},
		func() string { return "mem-1" },
		"worker-1",
	)
	worker.now = func() time.Time { return time.Unix(500, 0).UTC() }

	_, err := jobs.Submit(context.Background(), ingestjob.Job{
		ID:        "job-3",
		Content:   "remember this",
		Mode:      ingestjob.ModeNormal,
		Scope:     memory.ScopeUser,
		State:     ingestjob.StatePending,
		CreatedAt: time.Unix(100, 0).UTC(),
		UpdatedAt: time.Unix(100, 0).UTC(),
	})
	require.NoError(t, err)

	err = worker.RunOnce(context.Background())
	require.Error(t, err)
	require.Empty(t, memories.items)

	job, getErr := jobs.GetByID(context.Background(), "job-3")
	require.NoError(t, getErr)
	require.Equal(t, ingestjob.StatePending, job.State)
	require.Equal(t, "boom", job.LastError)
}

type fakeResponse struct {
	text string
	err  error
}

type sequenceLLM struct {
	responses []fakeResponse
	index     int
}

func (s *sequenceLLM) GenerateText(_ context.Context, _ []llm.Message) (string, error) {
	if s.index >= len(s.responses) {
		return "", errors.New("no more llm responses")
	}
	resp := s.responses[s.index]
	s.index++
	return resp.text, resp.err
}

type fakeEmbedder struct {
	vectors map[string][]float32
	err     error
}

func (f fakeEmbedder) Embed(_ context.Context, content string) ([]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vectors[content], nil
}

type fakeRecallService struct {
	results map[string][]memory.RecallResult
}

func (f fakeRecallService) Recall(_ context.Context, input memory.RecallInput) ([]memory.RecallResult, error) {
	return f.results[input.Content], nil
}

type fakeTxManager struct {
	memories *memoryRepo
	jobs     *jobRepo
}

func (f *fakeTxManager) Run(ctx context.Context, fn func(memory.Repository, ingestjob.Repository) error) error {
	return fn(f.memories, f.jobs)
}

type memoryRepo struct {
	items map[string]memory.Memory
}

func newMemoryRepo() *memoryRepo { return &memoryRepo{items: map[string]memory.Memory{}} }

func (r *memoryRepo) Create(_ context.Context, m memory.Memory) (memory.Memory, error) {
	r.items[m.ID] = m
	return m, nil
}

func (r *memoryRepo) UpsertByContentHash(_ context.Context, m memory.Memory) (memory.Memory, error) {
	for id, item := range r.items {
		if item.ContentHash != m.ContentHash {
			continue
		}
		item.StoreCount += m.StoreCount
		item.Embedding = m.Embedding
		item.Kind = memory.PrimaryKind(m.Kinds)
		item.State = m.State
		item.UpdatedAt = m.UpdatedAt
		r.items[id] = item
		return item, nil
	}
	m.Kind = memory.PrimaryKind(m.Kinds)
	r.items[m.ID] = m
	return m, nil
}

func (r *memoryRepo) Update(_ context.Context, m memory.Memory) (memory.Memory, error) {
	m.Kind = memory.PrimaryKind(m.Kinds)
	r.items[m.ID] = m
	return m, nil
}

func (r *memoryRepo) Delete(_ context.Context, id string) error {
	delete(r.items, id)
	return nil
}

func (r *memoryRepo) GetByID(_ context.Context, id string) (memory.Memory, error) {
	item, ok := r.items[id]
	if !ok {
		return memory.Memory{}, memory.ErrNotFound
	}
	return item, nil
}

func (r *memoryRepo) List(_ context.Context, _ memory.ListInput) ([]memory.Memory, int64, error) {
	out := make([]memory.Memory, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, int64(len(out)), nil
}

func (r *memoryRepo) ListTopKinds(_ context.Context, _ int) ([]memory.KindCount, error) {
	return nil, nil
}

func (r *memoryRepo) Search(_ context.Context, _ string, _ int) ([]memory.Memory, error) {
	return nil, nil
}
func (r *memoryRepo) VectorSearch(_ context.Context, _ []float32, _ int) ([]memory.RecallCandidate, error) {
	return nil, nil
}
func (r *memoryRepo) FullTextSearch(_ context.Context, _ string, _ int) ([]memory.RecallCandidate, error) {
	return nil, nil
}

type jobRepo struct {
	items map[string]ingestjob.Job
}

func newJobRepo() *jobRepo { return &jobRepo{items: map[string]ingestjob.Job{}} }

func (r *jobRepo) Submit(_ context.Context, job ingestjob.Job) (ingestjob.Job, error) {
	r.items[job.ID] = job
	return job, nil
}

func (r *jobRepo) GetByID(_ context.Context, id string) (ingestjob.Job, error) {
	job, ok := r.items[id]
	if !ok {
		return ingestjob.Job{}, ingestjob.ErrNotFound
	}
	return job, nil
}

func (r *jobRepo) ClaimNext(_ context.Context, workerID string, now time.Time) (ingestjob.Job, error) {
	for id, job := range r.items {
		if job.State != ingestjob.StatePending {
			continue
		}
		if job.NextRunAt != nil && job.NextRunAt.After(now) {
			continue
		}
		job.State = ingestjob.StateRunning
		job.ExecuteCount++
		job.WorkerID = workerID
		job.LockedAt = ptrTime(now)
		job.UpdatedAt = now
		r.items[id] = job
		return job, nil
	}
	return ingestjob.Job{}, ingestjob.ErrNotFound
}

func (r *jobRepo) MarkRetry(_ context.Context, claimed ingestjob.Job, nextRunAt time.Time, lastError string, now time.Time) (ingestjob.Job, error) {
	job := r.items[claimed.ID]
	if job.State != ingestjob.StateRunning || job.ExecuteCount != claimed.ExecuteCount || job.WorkerID != claimed.WorkerID {
		return ingestjob.Job{}, ingestjob.ErrConflict
	}
	job.State = ingestjob.StatePending
	job.NextRunAt = ptrTime(nextRunAt)
	job.LastError = lastError
	job.LockedAt = nil
	job.WorkerID = ""
	job.UpdatedAt = now
	r.items[claimed.ID] = job
	return job, nil
}

func (r *jobRepo) MarkFailed(_ context.Context, claimed ingestjob.Job, lastError string, now time.Time) (ingestjob.Job, error) {
	job := r.items[claimed.ID]
	if job.State != ingestjob.StateRunning || job.ExecuteCount != claimed.ExecuteCount || job.WorkerID != claimed.WorkerID {
		return ingestjob.Job{}, ingestjob.ErrConflict
	}
	job.State = ingestjob.StateFailed
	job.LastError = lastError
	job.LockedAt = nil
	job.WorkerID = ""
	job.UpdatedAt = now
	r.items[claimed.ID] = job
	return job, nil
}

func (r *jobRepo) MarkSucceeded(_ context.Context, claimed ingestjob.Job, resultMemoryIDs []string, resultSummary string, now time.Time) (ingestjob.Job, error) {
	job := r.items[claimed.ID]
	if job.State != ingestjob.StateRunning || job.ExecuteCount != claimed.ExecuteCount || job.WorkerID != claimed.WorkerID {
		return ingestjob.Job{}, ingestjob.ErrConflict
	}
	job.State = ingestjob.StateSucceeded
	job.ResultMemoryIDs = append([]string(nil), resultMemoryIDs...)
	job.ResultSummary = resultSummary
	job.LastError = ""
	job.NextRunAt = nil
	job.LockedAt = nil
	job.WorkerID = ""
	job.UpdatedAt = now
	r.items[claimed.ID] = job
	return job, nil
}

func ptrTime(value time.Time) *time.Time { return &value }
