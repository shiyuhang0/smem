package ingest

import (
	"encoding/json"
	"fmt"
	"strings"

	"smem/apps/server/internal/domain/memory"
)

type extractionPayload struct {
	Memories []struct {
		Content string   `json:"content"`
		Type    string   `json:"type"`
		Kinds   []string `json:"kinds"`
	} `json:"memories"`
}

type fusionPayload struct {
	Actions []fusionAction `json:"actions"`
}

func parseExtractionPayload(raw string) ([]extractedMemory, error) {
	var payload extractionPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	limit := len(payload.Memories)
	if limit > maxExtractedMemories {
		limit = maxExtractedMemories
	}

	out := make([]extractedMemory, 0, limit)
	for _, item := range payload.Memories[:limit] {
		memType, err := parseMemoryType(item.Type)
		if err != nil {
			return nil, err
		}
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		out = append(out, extractedMemory{
			Content: content,
			Type:    memType,
			Kinds:   filterKinds(item.Kinds),
		})
	}
	return out, nil
}

func parseFusionPayload(raw string, candidates []candidateMemory, recalled []memory.Memory) ([]fusionAction, error) {
	var payload fusionPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	expected := len(candidates) + len(recalled)
	if len(payload.Actions) != expected {
		return nil, fmt.Errorf("fusion action count mismatch: got %d want %d", len(payload.Actions), expected)
	}

	for i, candidate := range candidates {
		action := payload.Actions[i]
		if action.Target != "candidate" || action.ID != candidate.ID {
			return nil, fmt.Errorf("fusion candidate action order mismatch for %s", candidate.ID)
		}
		if action.Action != "ignore" && action.Action != "create" {
			return nil, fmt.Errorf("invalid candidate action %q", action.Action)
		}
		if action.Action == "create" {
			if action.Memory == nil || strings.TrimSpace(action.Memory.Content) == "" {
				return nil, fmt.Errorf("candidate create must include memory.content")
			}
		} else if action.Memory != nil {
			return nil, fmt.Errorf("candidate ignore must not include memory")
		}
	}

	for i, recalledMemory := range recalled {
		action := payload.Actions[len(candidates)+i]
		if action.Target != "memory" || action.ID != recalledMemory.ID {
			return nil, fmt.Errorf("fusion memory action order mismatch for %s", recalledMemory.ID)
		}
		switch action.Action {
		case "update":
			if action.Memory == nil || strings.TrimSpace(action.Memory.Content) == "" {
				return nil, fmt.Errorf("memory update must include memory.content")
			}
		case "delete", "ignore":
			if action.Memory != nil {
				return nil, fmt.Errorf("memory %s must not include memory payload", action.Action)
			}
		default:
			return nil, fmt.Errorf("invalid memory action %q", action.Action)
		}
	}

	return payload.Actions, nil
}

func parseMemoryType(value string) (memory.Type, error) {
	memType := memory.Type(strings.TrimSpace(value))
	switch memType {
	case memory.TypeUnknown, memory.TypeFact, memory.TypeEpisodic, memory.TypeProcedural:
		return memType, nil
	default:
		return memory.TypeUnknown, fmt.Errorf("invalid memory type %q", value)
	}
}

func filterKinds(input []string) []string {
	out := make([]string, 0, len(input))
	for _, kind := range input {
		kind = strings.TrimSpace(kind)
		if _, ok := allowedKinds[kind]; ok {
			out = append(out, kind)
		}
	}
	return out
}
