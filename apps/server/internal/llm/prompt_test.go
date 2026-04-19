package llm

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewExtractionPromptMatchesApprovedDocumentText(t *testing.T) {
	prompt := NewExtractionPrompt("hello world")
	require.Len(t, prompt, 2)
	require.Equal(t, extractionSystemPrompt, prompt[0].Content)
	require.Equal(t, strings.ReplaceAll(extractionUserPromptTemplate, "{{content}}", "hello world"), prompt[1].Content)
}

func TestNewFusionDecisionPromptMatchesApprovedDocumentText(t *testing.T) {
	candidates := []FusionCandidate{{ID: "c1", Content: "candidate text"}}
	recalled := []FusionMemory{{ID: "m1", Content: "memory text"}}

	candidateJSON, err := json.Marshal(candidates)
	require.NoError(t, err)
	recalledJSON, err := json.Marshal(recalled)
	require.NoError(t, err)

	prompt := NewFusionDecisionPrompt(candidates, recalled)
	require.Len(t, prompt, 2)
	require.Equal(t, fusionSystemPrompt, prompt[0].Content)

	expectedUser := strings.ReplaceAll(fusionUserPromptTemplate, "{{candidate_memories}}", string(candidateJSON))
	expectedUser = strings.ReplaceAll(expectedUser, "{{recalled_memories}}", string(recalledJSON))
	require.Equal(t, expectedUser, prompt[1].Content)
}
