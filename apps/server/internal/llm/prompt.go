package llm

import (
	"encoding/json"
	"strings"
)

func NewExtractionPrompt(content string) []Message {
	return []Message{
		{Role: "system", Content: extractionSystemPrompt},
		{Role: "user", Content: strings.ReplaceAll(extractionUserPromptTemplate, "{{content}}", content)},
	}
}

func NewRecallRewritePrompt(content string) []Message {
	return []Message{{Role: "system", Content: "Rewrite the query for semantic memory recall."}, {Role: "user", Content: content}}
}

type FusionCandidate struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type FusionMemory struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

func NewFusionDecisionPrompt(candidates []FusionCandidate, recalled []FusionMemory) []Message {
	candidateJSON, _ := json.Marshal(candidates)
	recalledJSON, _ := json.Marshal(recalled)

	content := strings.ReplaceAll(fusionUserPromptTemplate, "{{candidate_memories}}", string(candidateJSON))
	content = strings.ReplaceAll(content, "{{recalled_memories}}", string(recalledJSON))

	return []Message{
		{Role: "system", Content: fusionSystemPrompt},
		{Role: "user", Content: content},
	}
}

const extractionSystemPrompt = "You extract candidate memories from text.\n\n" +
	"Your job is to turn the input text into up to 5 candidate memories for long-term storage.\n" +
	"Each candidate memory must be small, atomic, and independently retrievable.\n\n" +
	"## What to return\n\n" +
	"Return a JSON object with a `memories` array. Each item may contain only:\n\n" +
	"- `content`: the final memory text\n" +
	"- `type`: one of `fact`, `episodic`, `procedural`, or `\"\"`\n" +
	"- `kinds`: an array of zero or more values from `skill`, `task`, `lesson`, `workflow`, `preference`, `profile`, `note`, `decision`\n\n" +
	"## Extraction policy\n\n" +
	"1. Extract only information supported by the input. Do not infer missing facts.\n" +
	"2. Keep memories durable and reusable. Good candidates include facts, preferences, decisions, working rules, plans, lessons, and meaningful background context.\n" +
	"3. Prefer one memory per idea. Do not collapse a whole conversation into one broad summary.\n" +
	"4. If two details are tightly coupled and splitting them would lose meaning, keep them in the same memory.\n" +
	"5. Prefer specific statements over vague summaries.\n" +
	"6. Ignore greetings, filler, transient chatter, and other content with no lasting value.\n" +
	"7. Do not store pure lookup or search intent such as \"what is X\" or \"how do I do Y\". If such a request also reveals stable background context, extract only the background context.\n" +
	"8. Preserve the original language of the input. Do not translate.\n" +
	"9. Preserve temporal wording as written. Keep expressions like \"tomorrow\" or \"next week\" instead of resolving them to calendar dates.\n" +
	"10. Make each memory self-contained. Avoid unclear pronouns when the referent is obvious from the input.\n" +
	"11. Remove duplicates within the candidate set. If two candidates overlap, keep the more specific one.\n" +
	"12. Return at most 5 memories. If nothing is worth storing, return an empty array.\n\n" +
	"## Type guidance\n\n" +
	"- `fact`: stable facts, profile data, background information, objective preferences\n" +
	"- `episodic`: events, experiences, or time-anchored happenings\n" +
	"- `procedural`: workflows, habits, operating rules, learned practices, or how-to knowledge\n\n" +
	"## Kind guidance\n\n" +
	"- `skill`: an ability or area of expertise\n" +
	"- `task`: an active task, TODO, or ongoing piece of work\n" +
	"- `lesson`: a lesson learned or postmortem-style takeaway\n" +
	"- `workflow`: a repeatable process or operating sequence\n" +
	"- `preference`: a preference, dislike, or setting choice\n" +
	"- `profile`: identity, role, relationship, or background information\n" +
	"- `note`: important context that does not fit the other kinds well\n" +
	"- `decision`: a chosen direction, conclusion, or explicit decision\n\n" +
	"`type` may be `\"\"` when no type fits confidently.\n" +
	"`kinds` may be `[]` when no kind fits confidently.\n" +
	"`kinds` may contain multiple values when a memory clearly belongs to more than one category.\n\n" +
	"## Example\n\n" +
	"Input:\n" +
	"\"I prefer using Go for backend services. I decided that new internal APIs should stay REST-first. I need to revisit the onboarding flow next week.\"\n\n" +
	"Output:\n" +
	"{\n" +
	"  \"memories\": [\n" +
	"    {\n" +
	"      \"content\": \"Prefers using Go for backend services\",\n" +
	"      \"type\": \"fact\",\n" +
	"      \"kinds\": [\"preference\"]\n" +
	"    },\n" +
	"    {\n" +
	"      \"content\": \"Decided that new internal APIs should stay REST-first\",\n" +
	"      \"type\": \"procedural\",\n" +
	"      \"kinds\": [\"decision\", \"workflow\"]\n" +
	"    },\n" +
	"    {\n" +
	"      \"content\": \"Needs to revisit the onboarding flow next week\",\n" +
	"      \"type\": \"\",\n" +
	"      \"kinds\": []\n" +
	"    }\n" +
	"  ]\n" +
	"}\n\n" +
	"## Output rules\n\n" +
	"Return valid JSON only. No markdown. No explanation.\n\n" +
	"{\n" +
	"  \"memories\": [\n" +
	"    {\n" +
	"      \"content\": \"...\",\n" +
	"      \"type\": \"fact\",\n" +
	"      \"kinds\": [\"preference\"]\n" +
	"    }\n" +
	"  ]\n" +
	"}"

const extractionUserPromptTemplate = "Extract candidate memories from the following input.\n\n" +
	"Requirements:\n" +
	"- Return at most 5 memories\n" +
	"- Keep each memory as atomic as possible\n" +
	"- Preserve the original language\n" +
	"- Return JSON only\n" +
	"- If nothing should be stored, return `{\"memories\":[]}`\n\n" +
	"Input:\n" +
	"{{content}}"

const fusionSystemPrompt = "You reconcile candidate memories against existing memories.\n\n" +
	"Your job is compare candidate memories with recalled existing memories and decide the final action for each one.\n\n" +
	"## Inputs\n\n" +
	"You will receive:\n\n" +
	"1. `candidate_memories`: extracted candidates, each with `id` and `content`\n" +
	"2. `recalled_memories`: relevant existing memories for those candidates; each recalled memory includes at least `id` and `content`\n\n" +
	"## Allowed actions\n\n" +
	"- Candidate memories: `ignore`, `create`\n" +
	"- Existing memories: `update`, `delete`, `ignore`\n" +
	"- Do not output any other action.\n\n" +
	"## Decision policy\n\n" +
	"1. If a candidate says the same thing as an existing memory, or is a refinement, normalization, clarification, or extension of it, use `ignore` for the candidate and `update` for that memory. Important: an existing memory must still be `update` even if its final `content` does not change. Reaffirmed or absorbed memories still count as updates.\n" +
	"2. Use `create` only when the candidate cannot be cleanly absorbed by any existing memory.\n" +
	"3. Use `delete` only when an existing memory is clearly contradicted or should be replaced rather than updated. Do not delete a memory merely because it is shorter, older, or less specific.\n" +
	"4. Use `ignore` only when a recalled memory should be fully left alone in this reconciliation. If the candidate reinforces, deepens, clarifies, or otherwise gets absorbed into that memory, use `update` instead. Use `ignore` only for memories that are not the right update target, do not conflict with any candidate, and should receive no content change or reinforcement at all.\n" +
	"5. Avoid duplicate creates. Preserve the original language. Do not translate or invent facts not grounded in the input.\n" +
	"6. Return exactly one action for each unique candidate id and each unique recalled memory id. \n" +
	"7. Return actions in a stable order: all candidate actions first in candidate input order, then all memory actions in first-seen memory id order.\n\n" +
	"## Output shape\n\n" +
	"Return a single `actions` array. Each action must include:\n\n" +
	"- `target`: `candidate` or `memory`\n" +
	"- `id`: the id of that candidate or existing memory\n" +
	"- `action`: one allowed action for that target\n\n" +
	"For every `create` or `update`, include:\n\n" +
	"- `memory.content`: final memory text\n\n" +
	"For `candidate` + `ignore`, you may include `absorbed_by_memory_ids`. This optional field is an array of memory ids. Use `[]` when no absorbed-by memory can be identified confidently. In most cases it should contain only one id, but it may contain multiple ids when the candidate is absorbed across multiple existing memories.\n\n" +
	"For `delete` or `ignore`, do not include `memory`.\n\n" +
	"## Output rules\n\n" +
	"Return valid JSON only. No markdown. No explanation.\n\n" +
	"{\n" +
	"  \"actions\": [\n" +
	"    {\n" +
	"      \"target\": \"candidate\",\n" +
	"      \"id\": \"1\",\n" +
	"      \"action\": \"ignore\",\n" +
	"      \"absorbed_by_memory_ids\": [\"mem_123\"]\n" +
	"    },\n" +
	"    {\n" +
	"      \"target\": \"candidate\",\n" +
	"      \"id\": \"2\",\n" +
	"      \"action\": \"create\",\n" +
	"      \"memory\": {\n" +
	"        \"content\": \"...\"\n" +
	"      }\n" +
	"    },\n" +
	"    {\n" +
	"      \"target\": \"memory\",\n" +
	"      \"id\": \"mem_123\",\n" +
	"      \"action\": \"update\",\n" +
	"      \"memory\": {\n" +
	"        \"content\": \"...\"\n" +
	"      }\n" +
	"    },\n" +
	"    {\n" +
	"      \"target\": \"memory\",\n" +
	"      \"id\": \"mem_456\",\n" +
	"      \"action\": \"delete\"\n" +
	"    },\n" +
	"    {\n" +
	"      \"target\": \"memory\",\n" +
	"      \"id\": \"mem_789\",\n" +
	"      \"action\": \"ignore\"\n" +
	"    }\n" +
	"  ]\n" +
	"}\n\n" +
	"## Examples\n\n" +
	"Example 1 - create new information\n\n" +
	"Candidate memories:\n" +
	"[{\"id\":\"c1\",\"content\":\"Sarah lives in Osaka\"}]\n\n" +
	"Recalled existing memories:\n" +
	"[{\"id\":\"m1\",\"content\":\"Sarah is my sister\"},{\"id\":\"m2\",\"content\":\"Is a software engineer\"}]\n\n" +
	"Result:\n" +
	"{\n" +
	"  \"actions\": [\n" +
	"    {\"target\":\"candidate\",\"id\":\"c1\",\"action\":\"create\",\"memory\":{\"content\":\"Sarah lives in Osaka\"}},\n" +
	"    {\"target\":\"memory\",\"id\":\"m1\",\"action\":\"ignore\"},\n" +
	"    {\"target\":\"memory\",\"id\":\"m2\",\"action\":\"ignore\"}\n" +
	"  ]\n" +
	"}\n\n" +
	"Example 2 - create the replacement and delete contradicted information:\n\n" +
	"Candidate memories:\n" +
	"[{\"id\":\"c1\",\"content\":\"Dislikes cheese pizza\"}]\n\n" +
	"Recalled existing memories:\n" +
	"[{\"id\":\"m1\",\"content\":\"Name is John\"},{\"id\":\"m2\",\"content\":\"Loves cheese pizza\"}]\n\n" +
	"Result:\n" +
	"{\n" +
	"  \"actions\": [\n" +
	"    {\"target\":\"candidate\",\"id\":\"c1\",\"action\":\"create\",\"memory\":{\"content\":\"Dislikes cheese pizza\"}},\n" +
	"    {\"target\":\"memory\",\"id\":\"m1\",\"action\":\"ignore\"},\n" +
	"    {\"target\":\"memory\",\"id\":\"m2\",\"action\":\"delete\"}\n" +
	"  ]\n" +
	"}\n\n" +
	"Example 3 - update an existing memory with richer detail:\n\n" +
	"Candidate memories:\n" +
	"[{\"id\":\"c1\",\"content\":\"Loves to play cricket with friends\"}]\n\n" +
	"Recalled existing memories:\n" +
	"[{\"id\":\"m1\",\"content\":\"User likes to play cricket\"},{\"id\":\"m2\",\"content\":\"User is a software engineer\"}]\n\n" +
	"Result:\n" +
	"{\n" +
	"  \"actions\": [\n" +
	"    {\"target\":\"candidate\",\"id\":\"c1\",\"action\":\"ignore\",\"absorbed_by_memory_ids\":[\"m1\"]},\n" +
	"    {\"target\":\"memory\",\"id\":\"m1\",\"action\":\"update\",\"memory\":{\"content\":\"Loves to play cricket with friends\"}},\n" +
	"    {\"target\":\"memory\",\"id\":\"m2\",\"action\":\"ignore\"}\n" +
	"  ]\n" +
	"}\n\n" +
	"Example 4 - same information or slight paraphrase still requires memory update:\n\n" +
	"Candidate memories:\n" +
	"[{\"id\":\"c1\",\"content\":\"Name is John\"},{\"id\":\"c2\",\"content\":\"Enjoys coffee\"}]\n\n" +
	"Recalled existing memories:\n" +
	"[{\"id\":\"m1\",\"content\":\"Name is John\"},{\"id\":\"m2\",\"content\":\"Likes coffee\"}]\n\n" +
	"Result:\n" +
	"{\n" +
	"  \"actions\": [\n" +
	"    {\"target\":\"candidate\",\"id\":\"c1\",\"action\":\"ignore\",\"absorbed_by_memory_ids\":[\"m1\"]},\n" +
	"    {\"target\":\"memory\",\"id\":\"m1\",\"action\":\"update\",\"memory\":{\"content\":\"Name is John\"}},\n" +
	"    {\"target\":\"candidate\",\"id\":\"c2\",\"action\":\"ignore\",\"absorbed_by_memory_ids\":[\"m2\"]},\n" +
	"    {\"target\":\"memory\",\"id\":\"m2\",\"action\":\"update\",\"memory\":{\"content\":\"Likes coffee\"}}\n" +
	"  ]\n" +
	"}\n\n" +
	"Example 5 - age may help choose an update target when there is a true conflict:\n\n" +
	"Candidate memories:\n" +
	"[{\"id\":\"c1\",\"content\":\"Prefers VS Code\"},{\"id\":\"c2\",\"content\":\"Works at company Y\"}]\n\n" +
	"Recalled existing memories:\n" +
	"[{\"id\":\"m1\",\"content\":\"Prefers vim\"},{\"id\":\"m2\",\"content\":\"Works at startup X\"}]\n\n" +
	"Result:\n" +
	"{\n" +
	"  \"actions\": [\n" +
	"    {\"target\":\"candidate\",\"id\":\"c1\",\"action\":\"ignore\",\"absorbed_by_memory_ids\":[\"m1\"]},\n" +
	"    {\"target\":\"candidate\",\"id\":\"c2\",\"action\":\"ignore\",\"absorbed_by_memory_ids\":[\"m2\"]},\n" +
	"    {\"target\":\"memory\",\"id\":\"m1\",\"action\":\"update\",\"memory\":{\"content\":\"Prefers VS Code\"}},\n" +
	"    {\"target\":\"memory\",\"id\":\"m2\",\"action\":\"update\",\"memory\":{\"content\":\"Works at company Y\"}}\n" +
	"  ]\n" +
	"}"

const fusionUserPromptTemplate = "Reconcile the candidate memories against the recalled existing memories.\n\n" +
	"Requirements:\n" +
	"- Follow the system rules\n" +
	"- Return exactly one action for each candidate id and each unique recalled memory id\n" +
	"- Return JSON only\n\n" +
	"Candidate memories:\n" +
	"{{candidate_memories}}\n\n" +
	"Recalled existing memories:\n" +
	"{{recalled_memories}}"
