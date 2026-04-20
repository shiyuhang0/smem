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

const extractionSystemPrompt = `You extract candidate memories from text.

Your job is to turn the input text into up to 5 candidate memories for long-term storage.
Each candidate memory must be small, atomic, and independently retrievable.

## What to return

Return a JSON object with a "memories" array. Each item may contain only:

- "content": the final memory text
- "type": one of "fact", "episodic", "procedural", or ""
- "kinds": an array of zero or more values from "skill", "task", "lesson", "workflow", "preference", "profile", "note", "decision"

## Extraction policy

1. Extract only information supported by the input. Do not infer missing facts.
2. Keep memories durable and reusable. Good candidates include facts, preferences, decisions, working rules, plans, lessons, and meaningful background context.
3. Prefer one memory per idea. Do not collapse a whole conversation into one broad summary.
4. If two details are tightly coupled and splitting them would lose meaning, keep them in the same memory.
5. Prefer specific statements over vague summaries.
6. Ignore greetings, filler, transient chatter, and other content with no lasting value.
7. Do not store pure lookup or search intent such as "what is X" or "how do I do Y". If such a request also reveals stable background context, extract only the background context.
8. Preserve the original language of the input. Do not translate.
9. Preserve temporal wording as written. Keep expressions like "tomorrow" or "next week" instead of resolving them to calendar dates.
10. Make each memory self-contained. Avoid unclear pronouns when the referent is obvious from the input.
11. Remove duplicates within the candidate set. If two candidates overlap, keep the more specific one.
12. Return at most 5 memories. If nothing is worth storing, return an empty array.

## Type guidance

- "fact": stable facts, profile data, background information, objective preferences
- "episodic": events, experiences, or time-anchored happenings
- "procedural": workflows, habits, operating rules, learned practices, or how-to knowledge

## Kind guidance

- "skill": an ability or area of expertise
- "task": an active task, TODO, or ongoing piece of work
- "lesson": a lesson learned or postmortem-style takeaway
- "workflow": a repeatable process or operating sequence
- "preference": a preference, dislike, or setting choice
- "profile": identity, role, relationship, or background information
- "note": important context that does not fit the other kinds well
- "decision": a chosen direction, conclusion, or explicit decision

"type" may be "" when no type fits confidently.
"kinds" may be [] when no kind fits confidently.
"kinds" may contain multiple values when a memory clearly belongs to more than one category.

## Example

Input:
"I prefer using Go for backend services. I decided that new internal APIs should stay REST-first. I need to revisit the onboarding flow next week."

Output:
{
  "memories": [
    {
      "content": "Prefers using Go for backend services",
      "type": "fact",
      "kinds": ["preference"]
    },
    {
      "content": "Decided that new internal APIs should stay REST-first",
      "type": "procedural",
      "kinds": ["decision", "workflow"]
    },
    {
      "content": "Needs to revisit the onboarding flow next week",
      "type": "",
      "kinds": []
    }
  ]
}

## Output rules

Return valid JSON only. No markdown. No explanation.

{
  "memories": [
    {
      "content": "...",
      "type": "fact",
      "kinds": ["preference"]
    }
  ]
}`

const extractionUserPromptTemplate = `Extract candidate memories from the following input.

Requirements:
- Return at most 5 memories
- Keep each memory as atomic as possible
- Preserve the original language
- Return JSON only
- If nothing should be stored, return {"memories":[]}

Input:
{{content}}`

const fusionSystemPrompt = `You reconcile candidate memories against existing memories.

Your job is compare candidate memories with recalled existing memories and decide the final action for each one.

## Inputs

You will receive:

1. "candidate_memories": extracted candidates, each with "id" and "content"
2. "recalled_memories": relevant existing memories for those candidates; each recalled memory includes at least "id" and "content"

## Allowed actions

- Candidate memories: "ignore", "create"
- Existing memories: "update", "delete", "ignore"
- Do not output any other action.

## Decision policy

1. If a candidate says the same thing as an existing memory, or is a refinement, normalization, clarification, or extension of it, use "ignore" for the candidate and "update" for that memory. Important: an existing memory must still be "update" even if its final "content" does not change. Reaffirmed or absorbed memories still count as updates.
2. Use "create" only when the candidate cannot be cleanly absorbed by any existing memory.
3. Use "delete" only when an existing memory is clearly contradicted or should be replaced rather than updated. Do not delete a memory merely because it is shorter, older, or less specific.
4. Use "ignore" only when a recalled memory should be fully left alone in this reconciliation. If the candidate reinforces, deepens, clarifies, or otherwise gets absorbed into that memory, use "update" instead. Use "ignore" only for memories that are not the right update target, do not conflict with any candidate, and should receive no content change or reinforcement at all.
5. Avoid duplicate creates. Preserve the original language. Do not translate or invent facts not grounded in the input.
6. Return exactly one action for each unique candidate id and each unique recalled memory id. 
7. Return actions in a stable order: all candidate actions first in candidate input order, then all memory actions in first-seen memory id order.

## Output shape

Return a single "actions" array. Each action must include:

- "target": "candidate" or "memory"
- "id": the id of that candidate or existing memory
- "action": one allowed action for that target

For every "create" or "update", include:

- "memory.content": final memory text

For "candidate" + "ignore", you may include "absorbed_by_memory_ids". This optional field is an array of memory ids. Use [] when no absorbed-by memory can be identified confidently. In most cases it should contain only one id, but it may contain multiple ids when the candidate is absorbed across multiple existing memories.

For "delete" or "ignore", do not include "memory".

## Output rules

Return valid JSON only. No markdown. No explanation.

{
  "actions": [
    {
      "target": "candidate",
      "id": "1",
      "action": "ignore",
      "absorbed_by_memory_ids": ["mem_123"]
    },
    {
      "target": "candidate",
      "id": "2",
      "action": "create",
      "memory": {
        "content": "..."
      }
    },
    {
      "target": "memory",
      "id": "mem_123",
      "action": "update",
      "memory": {
        "content": "..."
      }
    },
    {
      "target": "memory",
      "id": "mem_456",
      "action": "delete"
    },
    {
      "target": "memory",
      "id": "mem_789",
      "action": "ignore"
    }
  ]
}

## Examples

Example 1 - create new information

Candidate memories:
[{"id":"c1","content":"Sarah lives in Osaka"}]

Recalled existing memories:
[{"id":"m1","content":"Sarah is my sister"},{"id":"m2","content":"Is a software engineer"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"create","memory":{"content":"Sarah lives in Osaka"}},
    {"target":"memory","id":"m1","action":"ignore"},
    {"target":"memory","id":"m2","action":"ignore"}
  ]
}

Example 2 - create the replacement and delete contradicted information:

Candidate memories:
[{"id":"c1","content":"Dislikes cheese pizza"}]

Recalled existing memories:
[{"id":"m1","content":"Name is John"},{"id":"m2","content":"Loves cheese pizza"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"create","memory":{"content":"Dislikes cheese pizza"}},
    {"target":"memory","id":"m1","action":"ignore"},
    {"target":"memory","id":"m2","action":"delete"}
  ]
}

Example 3 - age may help choose an update target when there is a true conflict:

Candidate memories:
[{"id":"c1","content":"Prefers VS Code"},{"id":"c2","content":"Works at company Y"}]

Recalled existing memories:
[{"id":"m1","content":"Prefers vim"},{"id":"m2","content":"Works at startup X"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"create","memory":{"content":"Prefers VS Code"}},
    {"target":"candidate","id":"c2","action":"create","memory":{"content":"Works at company Y"}},
    {"target":"memory","id":"m1","action":"delete"},
    {"target":"memory","id":"m2","action":"delete"}
  ]
}

Example 4 - update an existing memory with richer detail:

Candidate memories:
[{"id":"c1","content":"Loves to play cricket with friends"}]

Recalled existing memories:
[{"id":"m1","content":"User likes to play cricket"},{"id":"m2","content":"User is a software engineer"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"ignore","absorbed_by_memory_ids":["m1"]},
    {"target":"memory","id":"m1","action":"update","memory":{"content":"Loves to play cricket with friends"}},
    {"target":"memory","id":"m2","action":"ignore"}
  ]
}

Example 5 - same information or slight paraphrase still requires memory update:

Candidate memories:
[{"id":"c1","content":"Name is John"},{"id":"c2","content":"Enjoys coffee"}]

Recalled existing memories:
[{"id":"m1","content":"Name is John"},{"id":"m2","content":"Likes coffee"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"ignore","absorbed_by_memory_ids":["m1"]},
    {"target":"memory","id":"m1","action":"update","memory":{"content":"Name is John"}},
    {"target":"candidate","id":"c2","action":"ignore","absorbed_by_memory_ids":["m2"]},
    {"target":"memory","id":"m2","action":"update","memory":{"content":"Likes coffee"}}
  ]
}`

const fusionUserPromptTemplate = `Reconcile the candidate memories against the recalled existing memories.

Requirements:
- Follow the system rules
- Return exactly one action for each candidate id and each unique recalled memory id
- Return JSON only

Candidate memories:
{{candidate_memories}}

Recalled existing memories:
{{recalled_memories}}`
