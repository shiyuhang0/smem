package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = fmt.Errorf("memory not found")

type Service struct {
	repo Repository
	now  func() time.Time
	id   func() string
}

func NewService(repo Repository, id func() string) *Service {
	return &Service{repo: repo, now: time.Now, id: id}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Memory, error) {
	if err := input.Validate(); err != nil {
		return Memory{}, err
	}

	item := s.buildMemory(input)
	return s.createOrUpdateExisting(ctx, item)
}

func (s *Service) Get(ctx context.Context, id string) (Memory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (Memory, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Memory{}, err
	}

	item, err = s.applyUpdateInput(item, input)
	if err != nil {
		return Memory{}, err
	}

	item.UpdatedAt = s.now().UTC()
	return s.repo.Update(ctx, item)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context, input ListInput) ([]Memory, int64, error) {
	input = normalizeListInput(input)
	return s.repo.List(ctx, input)
}

func (s *Service) buildMemory(input CreateInput) Memory {
	now := s.now().UTC()
	return Memory{
		ID:          s.id(),
		Content:     strings.TrimSpace(input.Content),
		ContentHash: HashContent(input.Content),
		Type:        input.Type,
		Kinds:       append([]string(nil), input.Kinds...),
		Scope:       defaultScope(input.Scope),
		State:       StateCreating,
		Metadata:    input.Metadata,
		AgentID:     input.AgentID,
		SessionID:   input.SessionID,
		Source:      input.Source,
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (s *Service) createOrUpdateExisting(ctx context.Context, item Memory) (Memory, error) {
	existing, err := s.repo.GetByContentHash(ctx, item.ContentHash)
	if err == nil {
		existing.StoreCount++
		existing.UpdatedAt = item.UpdatedAt
		return s.repo.Update(ctx, existing)
	}
	return s.repo.Create(ctx, item)
}

func (s *Service) applyUpdateInput(item Memory, input UpdateInput) (Memory, error) {
	if input.Content != nil {
		trimmed := strings.TrimSpace(*input.Content)
		if trimmed == "" {
			return Memory{}, fmt.Errorf("content is required")
		}
		item.Content = trimmed
		item.ContentHash = HashContent(trimmed)
		item.Version++
	}
	if input.Type != nil {
		if *input.Type != TypeUnknown && !isValidType(*input.Type) {
			return Memory{}, fmt.Errorf("type is invalid")
		}
		item.Type = *input.Type
	}
	if input.Scope != nil {
		if !isValidScope(*input.Scope) {
			return Memory{}, fmt.Errorf("scope is invalid")
		}
		item.Scope = *input.Scope
	}
	if input.State != nil {
		if !isValidState(*input.State) {
			return Memory{}, fmt.Errorf("state is invalid")
		}
		item.State = *input.State
	}
	if input.Kinds != nil {
		item.Kinds = append([]string(nil), input.Kinds...)
	}
	if input.Metadata != nil {
		item.Metadata = input.Metadata
	}
	return item, nil
}

func normalizeListInput(input ListInput) ListInput {
	if input.Page < 1 {
		input.Page = 1
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		input.PageSize = 20
	}
	return input
}

func HashContent(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

func defaultScope(scope Scope) Scope {
	if scope == "" {
		return ScopeUser
	}
	return scope
}
