package llm

import (
	"fmt"
	"strings"

	"smem/apps/server/internal/domain/memory"
)

func NewExtractionPrompt(content string) []Message {
	return []Message{{Role: "system", Content: "Extract atomic long-term memories."}, {Role: "user", Content: content}}
}

func NewRecallRewritePrompt(content string) []Message {
	return []Message{{Role: "system", Content: "Rewrite the query for semantic memory recall."}, {Role: "user", Content: content}}
}

func NewFusionDecisionPrompt(candidate memory.CreateInput, existing []memory.Memory) []Message {
	lines := make([]string, 0, len(existing))
	for _, item := range existing {
		lines = append(lines, fmt.Sprintf("id=%s content=%q type=%s kinds=%s", item.ID, item.Content, item.Type, strings.Join(item.Kinds, ",")))
	}
	content := fmt.Sprintf("candidate: %q\nexisting:\n%s", candidate.Content, strings.Join(lines, "\n"))
	return []Message{{Role: "system", Content: "Return JSON with decision ignore/create/update and optional memory_id/content."}, {Role: "user", Content: content}}
}
