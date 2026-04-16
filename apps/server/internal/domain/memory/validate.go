package memory

import (
	"fmt"
	"strings"
)

func (i CreateInput) Validate() error {
	if strings.TrimSpace(i.Content) == "" {
		return fmt.Errorf("content is required")
	}
	if i.Mode != ModeNormal && i.Mode != ModeSmart {
		return fmt.Errorf("mode must be normal or smart")
	}
	if i.Type != TypeUnknown && !isValidType(i.Type) {
		return fmt.Errorf("type is invalid")
	}
	if i.Scope != "" && !isValidScope(i.Scope) {
		return fmt.Errorf("scope is invalid")
	}
	return nil
}

func (i RecallInput) Validate() error {
	if strings.TrimSpace(i.Content) == "" {
		return fmt.Errorf("content is required")
	}
	if i.TopK < 1 || i.TopK > 10 {
		return fmt.Errorf("top_k must be between 1 and 10")
	}
	return nil
}

func isValidType(v Type) bool {
	switch v {
	case TypeFact, TypeEpisodic, TypeProcedural:
		return true
	default:
		return false
	}
}

func isValidScope(v Scope) bool {
	switch v {
	case ScopeUser, ScopeAgent, ScopeExternal:
		return true
	default:
		return false
	}
}

func isValidState(v State) bool {
	switch v {
	case StateCreating, StateActive, StateArchived:
		return true
	default:
		return false
	}
}
