package ingest

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/llm"
)

type Worker interface {
	Queue(context.Context, memory.Memory) error
}

type Service struct {
	memoryService *memory.Service
	repo          memory.Repository
	worker        Worker
	llm           llm.Provider
}

func NewService(memoryService *memory.Service, repo memory.Repository, worker Worker, llmProvider llm.Provider) *Service {
	return &Service{memoryService: memoryService, repo: repo, worker: worker, llm: llmProvider}
}

func (s *Service) Create(ctx context.Context, input memory.CreateInput) ([]memory.Memory, error) {
	if input.Mode == memory.ModeSmart {
		items, err := s.createSmart(ctx, input)
		if err == nil {
			return items, nil
		}
	}
	item, err := s.memoryService.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	if s.worker != nil {
		if err := s.worker.Queue(ctx, item); err != nil {
			return nil, err
		}
	}
	return []memory.Memory{item}, nil
}

func (s *Service) createSmart(ctx context.Context, input memory.CreateInput) ([]memory.Memory, error) {
	if s.llm == nil {
		return nil, context.Canceled
	}
	raw, err := s.llm.GenerateText(ctx, llm.NewExtractionPrompt(input.Content))
	if err != nil {
		return nil, err
	}
	var payload struct {
		Memories []struct {
			Content string   `json:"content"`
			Type    string   `json:"type"`
			Kinds   []string `json:"kinds"`
		} `json:"memories"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil || len(payload.Memories) == 0 {
		return nil, err
	}
	// max 10 memories
	if len(payload.Memories) > 10 {
		payload.Memories = payload.Memories[:10]
	}
	items := make([]memory.Memory, 0, len(payload.Memories))
	for _, candidate := range payload.Memories {
		candidateInput := memory.CreateInput{
			Content:   candidate.Content,
			Mode:      memory.ModeNormal,
			Type:      memory.Type(candidate.Type),
			Kinds:     candidate.Kinds,
			Scope:     input.Scope,
			Metadata:  input.Metadata,
			AgentID:   input.AgentID,
			SessionID: input.SessionID,
			Source:    input.Source,
		}
		resolved, err := s.applyFusionDecision(ctx, candidateInput)
		if err != nil {
			return nil, err
		}
		if resolved == nil {
			continue
		}
		if s.worker != nil {
			if err := s.worker.Queue(ctx, *resolved); err != nil {
				return nil, err
			}
		}
		items = append(items, *resolved)
	}
	return items, nil
}

func (s *Service) applyFusionDecision(ctx context.Context, candidate memory.CreateInput) (*memory.Memory, error) {
	existing, err := s.repo.Search(ctx, candidate.Content, 5)
	if err != nil {
		existing = nil
	}
	decision := struct {
		Decision string `json:"decision"`
		MemoryID string `json:"memory_id"`
		Content  string `json:"content"`
	}{Decision: "create"}
	if s.llm != nil {
		raw, err := s.llm.GenerateText(ctx, llm.NewFusionDecisionPrompt(candidate, existing))
		if err == nil {
			_ = json.Unmarshal([]byte(raw), &decision)
		}
	}
	switch strings.ToLower(strings.TrimSpace(decision.Decision)) {
	case "ignore":
		return nil, nil
	case "update":
		if decision.MemoryID == "" {
			break
		}
		item, err := s.repo.GetByID(ctx, decision.MemoryID)
		if err != nil {
			break
		}
		now := time.Now().UTC()
		item.StoreCount++
		item.UpdatedAt = now
		if content := strings.TrimSpace(decision.Content); content != "" && content != item.Content {
			item.Content = content
			item.ContentHash = memory.HashContent(content)
			item.Version++
		}
		if candidate.Type != memory.TypeUnknown {
			item.Type = candidate.Type
		}
		if len(candidate.Kinds) > 0 {
			item.Kinds = append([]string(nil), candidate.Kinds...)
		}
		updated, err := s.repo.Update(ctx, item)
		if err != nil {
			return nil, err
		}
		return &updated, nil
	}
	created, err := s.memoryService.Create(ctx, candidate)
	if err != nil {
		return nil, err
	}
	return &created, nil
}
