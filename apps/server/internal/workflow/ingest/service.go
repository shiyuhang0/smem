package ingest

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/llm"
	"smem/apps/server/internal/observability"
)

type Worker interface {
	Queue(context.Context, memory.Memory) error
}

var ingestLogger = observability.NewLogger("[ingest] ")

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
	if input.Mode != memory.ModeSmart {
		return s.createNormal(ctx, input)
	}

	items, err := s.createSmart(ctx, input)
	if err == nil {
		return items, nil
	}

	ingestLogger.Printf("smart_create_fallback content=%q err=%v", input.Content, err)
	return s.createNormal(ctx, input)
}

func (s *Service) createNormal(ctx context.Context, input memory.CreateInput) ([]memory.Memory, error) {
	item, err := s.memoryService.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.queueMemory(ctx, item); err != nil {
		return nil, err
	}
	ingestLogger.Printf("create_mode=normal memory_id=%s", item.ID)
	return []memory.Memory{item}, nil
}

func (s *Service) createSmart(ctx context.Context, input memory.CreateInput) ([]memory.Memory, error) {
	candidates, err := s.extractSmartCandidates(ctx, input)
	if err != nil {
		return nil, err
	}

	items := make([]memory.Memory, 0, len(candidates))
	for _, candidate := range candidates {
		resolved, err := s.applyFusionDecision(ctx, candidate)
		if err != nil {
			return nil, err
		}
		if resolved == nil {
			ingestLogger.Printf("fusion_decision=ignore content=%q", candidate.Content)
			continue
		}
		if err := s.queueMemory(ctx, *resolved); err != nil {
			return nil, err
		}
		ingestLogger.Printf("fusion_decision=store memory_id=%s content=%q", resolved.ID, resolved.Content)
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
		ingestLogger.Printf("fusion_decision=update memory_id=%s", updated.ID)
		return &updated, nil
	}

	created, err := s.memoryService.Create(ctx, candidate)
	if err != nil {
		return nil, err
	}
	ingestLogger.Printf("fusion_decision=create memory_id=%s", created.ID)
	return &created, nil
}

func (s *Service) extractSmartCandidates(ctx context.Context, input memory.CreateInput) ([]memory.CreateInput, error) {
	if s.llm == nil {
		return nil, context.Canceled
	}

	raw, err := s.llm.GenerateText(ctx, llm.NewExtractionPrompt(input.Content))
	if err != nil {
		return nil, err
	}

	payload, err := parseExtractionPayload(raw)
	if err != nil {
		return nil, err
	}

	candidates := make([]memory.CreateInput, 0, len(payload.Memories))
	for _, candidate := range limitExtractedMemories(payload.Memories) {
		candidates = append(candidates, memory.CreateInput{
			Content:   candidate.Content,
			Mode:      memory.ModeNormal,
			Type:      memory.Type(candidate.Type),
			Kinds:     candidate.Kinds,
			Scope:     input.Scope,
			Metadata:  input.Metadata,
			AgentID:   input.AgentID,
			SessionID: input.SessionID,
			Source:    input.Source,
		})
	}
	return candidates, nil
}

func (s *Service) queueMemory(ctx context.Context, item memory.Memory) error {
	if s.worker == nil {
		return nil
	}
	return s.worker.Queue(ctx, item)
}

type extractionPayload struct {
	Memories []struct {
		Content string   `json:"content"`
		Type    string   `json:"type"`
		Kinds   []string `json:"kinds"`
	} `json:"memories"`
}

func parseExtractionPayload(raw string) (extractionPayload, error) {
	var payload extractionPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return extractionPayload{}, err
	}
	if len(payload.Memories) == 0 {
		return extractionPayload{}, nil
	}
	return payload, nil
}

func limitExtractedMemories(items []struct {
	Content string   `json:"content"`
	Type    string   `json:"type"`
	Kinds   []string `json:"kinds"`
}) []struct {
	Content string   `json:"content"`
	Type    string   `json:"type"`
	Kinds   []string `json:"kinds"`
} {
	// Smart extraction should stay focused even if the LLM returns a long list.
	if len(items) > 10 {
		return items[:10]
	}
	return items
}
