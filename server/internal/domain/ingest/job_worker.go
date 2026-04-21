package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"smem/apps/server/internal/ai/embedding"
	"smem/apps/server/internal/ai/llm"
	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"
)

type recallService interface {
	Recall(context.Context, memory.RecallInput) ([]memory.RecallResult, error)
}

type transactionManager interface {
	Run(context.Context, func(memory.Repository, ingestjob.Repository) error) error
}

type JobWorker struct {
	jobs         ingestjob.Repository
	tx           transactionManager
	recall       recallService
	llm          llm.Provider
	embedder     embedding.Provider
	memoryID     func() string
	now          func() time.Time
	pollInterval time.Duration
	workerID     string
}

func NewJobWorker(
	jobs ingestjob.Repository,
	tx transactionManager,
	recall recallService,
	llmProvider llm.Provider,
	embedder embedding.Provider,
	memoryID func() string,
	workerID string,
) *JobWorker {
	return &JobWorker{
		jobs:         jobs,
		tx:           tx,
		recall:       recall,
		llm:          llmProvider,
		embedder:     embedder,
		memoryID:     memoryID,
		now:          time.Now,
		pollInterval: defaultPollInterval,
		workerID:     workerID,
	}
}

func (w *JobWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := w.RunOnce(ctx); err != nil && err != ingestjob.ErrNotFound {
					ingestLogger.Printf("job_poll_error err=%v", err)
				}
			}
		}
	}()
}

func (w *JobWorker) RunOnce(ctx context.Context) error {
	ingestLogger.Printf("run ingest worker")
	job, err := w.jobs.ClaimNext(ctx, w.workerID, w.now().UTC())
	if err != nil {
		if err == ingestjob.ErrNotFound {
			return nil
		}
		return err
	}

	ingestLogger.Printf("job_claimed job_id=%s attempt=%d mode=%s", job.ID, job.ExecuteCount, job.Mode)

	if err := w.executeJob(ctx, job); err != nil {
		return w.handleFailure(ctx, job, err)
	}
	return nil
}

func (w *JobWorker) executeJob(ctx context.Context, job ingestjob.Job) error {
	switch job.Mode {
	case ingestjob.ModeNormal:
		return w.executeNormalJob(ctx, job)
	case ingestjob.ModeSmart:
		return w.executeSmartJob(ctx, job)
	default:
		return fmt.Errorf("unsupported ingest mode %q", job.Mode)
	}
}

func (w *JobWorker) executeNormalJob(ctx context.Context, job ingestjob.Job) error {
	now := w.now().UTC()
	item := memory.Memory{
		ID:          w.memoryID(),
		Content:     strings.TrimSpace(job.Content),
		ContentHash: memory.HashContent(job.Content),
		Type:        job.Type,
		Kinds:       append([]string(nil), job.Kinds...),
		Scope:       job.Scope,
		State:       memory.StateActive,
		Metadata:    job.Metadata,
		AgentID:     job.AgentID,
		SessionID:   job.SessionID,
		Source:      job.Source,
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	embeddingVector, err := w.embed(ctx, item.Content)
	if err != nil {
		return err
	}
	item.Embedding = embeddingVector

	writeSet := memoryWriteSet{
		creates:         []memory.Memory{item},
		resultSummary:   "created=1 updated=0 deleted=0",
		resultMemoryIDs: []string{item.ID},
	}
	return w.commitWriteSet(ctx, job.ID, writeSet)
}

func (w *JobWorker) executeSmartJob(ctx context.Context, job ingestjob.Job) error {
	candidates, err := w.extractCandidates(ctx, job)
	if err != nil {
		return err
	}
	ingestLogger.Printf("job_extracted job_id=%s candidates=%v", job.ID, candidates)
	if len(candidates) == 0 {
		return w.commitWriteSet(ctx, job.ID, memoryWriteSet{resultSummary: "created=0 updated=0 deleted=0"})
	}

	recalled, err := w.recallMemories(ctx, job.ID, candidates)
	if err != nil {
		return err
	}
	actions, err := w.fuseCandidates(ctx, job.ID, candidates, recalled)
	if err != nil {
		return err
	}
	ingestLogger.Printf("smart_fusion_actions job_id=%s actions=%v", job.ID, actions)
	writeSet, err := w.buildWriteSet(ctx, candidates, recalled, actions)
	if err != nil {
		return err
	}
	return w.commitWriteSet(ctx, job.ID, writeSet)
}

func (w *JobWorker) extractCandidates(ctx context.Context, job ingestjob.Job) ([]candidateMemory, error) {
	if w.llm == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	raw, err := w.llm.GenerateText(ctx, llm.NewExtractionPrompt(job.Content))
	if err != nil {
		return nil, err
	}
	ingestLogger.Printf("smart_extract_llm_response job_id=%s raw=%s", job.ID, raw)
	extracted, err := parseExtractionPayload(raw)
	if err != nil {
		return nil, err
	}

	candidates := make([]candidateMemory, 0, len(extracted))
	for idx, item := range extracted {
		candidates = append(candidates, candidateMemory{
			ID:        fmt.Sprintf("c%d", idx+1),
			Content:   item.Content,
			Type:      item.Type,
			Kinds:     append([]string(nil), item.Kinds...),
			Scope:     job.Scope,
			Metadata:  job.Metadata,
			AgentID:   job.AgentID,
			SessionID: job.SessionID,
			Source:    job.Source,
		})
	}
	return candidates, nil
}

func (w *JobWorker) recallMemories(ctx context.Context, jobID string, candidates []candidateMemory) ([]memory.Memory, error) {
	if w.recall == nil {
		return nil, nil
	}
	seen := map[string]struct{}{}
	out := make([]memory.Memory, 0)
	for _, candidate := range candidates {
		results, err := w.recall.Recall(ctx, memory.RecallInput{Content: candidate.Content, TopK: maxRecallPerCandidate})
		if err != nil {
			return nil, err
		}
		for _, result := range results {
			if _, ok := seen[result.Memory.ID]; ok {
				continue
			}
			seen[result.Memory.ID] = struct{}{}
			out = append(out, result.Memory)
		}
	}
	return out, nil
}

func (w *JobWorker) fuseCandidates(ctx context.Context, jobID string, candidates []candidateMemory, recalled []memory.Memory) ([]fusionAction, error) {
	if w.llm == nil {
		return nil, fmt.Errorf("llm provider is not configured")
	}
	promptCandidates := make([]llm.FusionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		promptCandidates = append(promptCandidates, llm.FusionCandidate{ID: candidate.ID, Content: candidate.Content})
	}
	promptMemories := make([]llm.FusionMemory, 0, len(recalled))
	for _, item := range recalled {
		promptMemories = append(promptMemories, llm.FusionMemory{ID: item.ID, Content: item.Content})
	}

	raw, err := w.llm.GenerateText(ctx, llm.NewFusionDecisionPrompt(promptCandidates, promptMemories))
	if err != nil {
		return nil, err
	}
	ingestLogger.Printf("smart_fusion_llm_response job_id=%s raw=%s", jobID, raw)
	actions, err := parseFusionPayload(raw, candidates, recalled)
	if err != nil {
		return nil, err
	}
	return actions, nil
}

func (w *JobWorker) buildWriteSet(ctx context.Context, candidates []candidateMemory, recalled []memory.Memory, actions []fusionAction) (memoryWriteSet, error) {
	now := w.now().UTC()
	candidateByID := make(map[string]candidateMemory, len(candidates))
	for _, candidate := range candidates {
		candidateByID[candidate.ID] = candidate
	}
	recalledByID := make(map[string]memory.Memory, len(recalled))
	for _, item := range recalled {
		recalledByID[item.ID] = item
	}

	writeSet := memoryWriteSet{}
	createCount := 0
	updateCount := 0
	deleteCount := 0

	for _, action := range actions {
		switch action.Target {
		case "candidate":
			if action.Action != "create" {
				continue
			}
			candidate := candidateByID[action.ID]
			item := memory.Memory{
				ID:          w.memoryID(),
				Content:     strings.TrimSpace(action.Memory.Content),
				ContentHash: memory.HashContent(action.Memory.Content),
				Type:        candidate.Type,
				Kinds:       append([]string(nil), candidate.Kinds...),
				Scope:       candidate.Scope,
				State:       memory.StateActive,
				Metadata:    candidate.Metadata,
				AgentID:     candidate.AgentID,
				SessionID:   candidate.SessionID,
				Source:      candidate.Source,
				Version:     1,
				StoreCount:  1,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			vector, err := w.embed(ctx, item.Content)
			if err != nil {
				return memoryWriteSet{}, err
			}
			item.Embedding = vector
			writeSet.creates = append(writeSet.creates, item)
			writeSet.resultMemoryIDs = append(writeSet.resultMemoryIDs, item.ID)
			createCount++
		case "memory":
			existing := recalledByID[action.ID]
			switch action.Action {
			case "update":
				updated := existing
				updated.StoreCount++
				updated.UpdatedAt = now
				newContent := strings.TrimSpace(action.Memory.Content)
				if newContent != updated.Content {
					updated.Content = newContent
					updated.ContentHash = memory.HashContent(newContent)
					updated.Version++
				}
				updated.State = memory.StateActive
				vector, err := w.embed(ctx, updated.Content)
				if err != nil {
					return memoryWriteSet{}, err
				}
				updated.Embedding = vector
				writeSet.updates = append(writeSet.updates, updated)
				writeSet.resultMemoryIDs = append(writeSet.resultMemoryIDs, updated.ID)
				updateCount++
			case "delete":
				writeSet.deletes = append(writeSet.deletes, existing.ID)
				deleteCount++
			}
		}
	}

	writeSet.resultSummary = fmt.Sprintf("created=%d updated=%d deleted=%d", createCount, updateCount, deleteCount)
	if len(writeSet.resultMemoryIDs) == 0 && createCount == 0 && updateCount == 0 && deleteCount == 0 {
		writeSet.resultSummary = "created=0 updated=0 deleted=0"
	}
	return writeSet, nil
}

func (w *JobWorker) embed(ctx context.Context, content string) ([]float32, error) {
	if w.embedder == nil {
		return nil, fmt.Errorf("embedding provider is not configured")
	}
	return w.embedder.Embed(ctx, content)
}

func (w *JobWorker) commitWriteSet(ctx context.Context, jobID string, writeSet memoryWriteSet) error {
	return w.tx.Run(ctx, func(memoryRepo memory.Repository, jobRepo ingestjob.Repository) error {
		for _, item := range writeSet.creates {
			stored, err := memoryRepo.UpsertByContentHash(ctx, item)
			if err != nil {
				return err
			}
			replaceResultMemoryID(&writeSet.resultMemoryIDs, item.ID, stored.ID)
		}
		for _, item := range writeSet.updates {
			if _, err := memoryRepo.Update(ctx, item); err != nil {
				return err
			}
		}
		for _, id := range writeSet.deletes {
			if err := memoryRepo.Delete(ctx, id); err != nil {
				return err
			}
		}
		currentJob, err := jobRepo.GetByID(ctx, jobID)
		if err != nil {
			return err
		}
		_, err = jobRepo.MarkSucceeded(ctx, currentJob, writeSet.resultMemoryIDs, writeSet.resultSummary, w.now().UTC())
		return err
	})
}

func replaceResultMemoryID(ids *[]string, oldID, newID string) {
	for i, id := range *ids {
		if id == oldID {
			(*ids)[i] = newID
		}
	}
}

func (w *JobWorker) handleFailure(ctx context.Context, job ingestjob.Job, err error) error {
	now := w.now().UTC()
	if job.ExecuteCount >= maxJobAttempts {
		_, updateErr := w.jobs.MarkFailed(ctx, job, err.Error(), now)
		if updateErr != nil {
			return updateErr
		}
		ingestLogger.Printf("job_failed job_id=%s err=%v", job.ID, err)
		return err
	}
	nextRunAt := now.Add(time.Duration(job.ExecuteCount) * time.Second)
	_, updateErr := w.jobs.MarkRetry(ctx, job, nextRunAt, err.Error(), now)
	if updateErr != nil {
		return updateErr
	}
	ingestLogger.Printf("job_retry_scheduled job_id=%s attempt=%d err=%v", job.ID, job.ExecuteCount, err)
	return err
}
