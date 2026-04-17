# Recall-Style Server Refactor Design

## Goal

Refactor `apps/server` so its core Go code follows the same style qualities as `internal/workflow/recall/service.go`: readable main flows, clear helper extraction, focused comments at decision points, and stable observability at key stages.

## Non-Goals

- Changing product behavior or API contracts.
- Re-architecting package boundaries across `workflow`, `domain`, `store`, `llm`, and `embedding`.
- Introducing a new logging framework or cross-cutting middleware abstraction.
- Splitting files aggressively when the existing file is already easy to read.

## Source Style To Generalize

The `Recall` method provides the target style for this refactor:

1. Normalize defaults at the top of the entrypoint.
2. Keep the main flow linear and easy to scan.
3. Extract non-trivial steps into clearly named helpers.
4. Add comments only where the code needs intent, trade-off, or future-direction context.
5. Log stage outputs so debugging does not require stepping through the code.

These traits should be generalized without copying the exact shape of `Recall` into places where it does not fit.

## Scope

This refactor applies to `apps/server` only.

### Priority 1: Core Workflow And Service Flows

- `internal/workflow/recall`
- `internal/workflow/ingest`
- `internal/domain/memory`

These packages contain the clearest request-to-result flows and should become the style reference for the rest of the server.

### Priority 2: Persistence And External Integrations

- `internal/store/tidb`
- `internal/llm`
- `internal/embedding`

These packages should follow the same readability rules while preserving existing retry, persistence, and provider behavior.

### Priority 3: Supporting Packages

- `internal/search`
- `internal/retry`
- nearby focused utilities touched by the refactor

These packages should only receive lightweight cleanup needed to keep style consistent with the higher-priority code.

## Design Rules

### 1. Main Methods Stay Linear

Public entrypoints and important orchestration methods should read top-to-bottom as a sequence of stages:

- normalize input
- prepare stage data
- call dependencies
- log important outputs
- finalize result

The reader should be able to understand the high-level behavior without expanding helper bodies.

### 2. Helper Methods Own Details

If a block performs one meaningful subtask, move it into a helper with an intention-revealing name. Good helper boundaries include:

- input normalization
- candidate loading
- decision application
- score calculation
- query construction
- row scanning and mapping
- provider request/response handling

Helpers should stay close to their caller unless broader reuse is already clear.

### 3. Comments Explain Decisions, Not Mechanics

Allowed comment patterns:

- why a fallback exists
- why a branch is intentionally conservative
- why a ranking or retry rule exists
- what future extension point a placeholder represents

Disallowed comment patterns:

- restating the next line of code
- narrating simple assignments
- commenting every helper when its name is already clear

### 4. Logs Appear At Stage Boundaries

Critical flows should log:

- stage input summary when useful
- stage output summary
- fallback or degradation decisions
- final result summary

Logging must remain concise and should prefer summarized values over full payload dumps. Internal hot loops should not gain noisy per-item logs unless the loop itself is the diagnostic target.

### 5. Behavior-Preserving Refactor First

This project-wide pass is a style refactor, not a feature pass. Allowed changes:

- method extraction
- local renaming for clarity
- local variable reshaping
- log additions
- comment cleanup
- file-local helper reordering
- splitting a large file only when it materially improves readability and keeps package boundaries intact

Disallowed changes unless separately approved:

- business rule changes
- API shape changes
- storage schema changes
- retry policy changes
- ranking formula changes
- broad renaming across packages with no readability payoff

## Package-Level Application

### `internal/workflow/recall`

Use this package as the baseline style reference. Refactor surrounding helpers so they support the `Recall` entrypoint style rather than obscuring it. Preserve existing ranking behavior unless a bug is discovered and explicitly addressed.

### `internal/workflow/ingest`

Make `Create`, smart-ingest, and fusion-decision flows read as clear stage pipelines. Pull apart extraction, candidate shaping, fusion lookup, decision parsing, and queue dispatch into helpers where needed. Add logs around smart-ingest fallback and decision outcomes.

### `internal/domain/memory`

Make service methods consistently structured:

- validate or normalize first
- build domain object or update patch
- perform repository call
- return clearly

Keep domain rules centralized and avoid mixing formatting noise into the main service methods.

### `internal/store/tidb`

Repository methods should expose a clean shape:

- normalize query inputs
- build query
- execute query
- map result

Consolidate repeated row scanning and query preparation where that improves readability. Keep GORM and SQL details out of higher-level call sites.

### `internal/llm` And `internal/embedding`

Provider methods should clearly show:

- request construction
- retry-wrapped execution
- response validation
- output mapping

Keep shared retry semantics unchanged. Add contextual errors and lightweight logs only where they help diagnose provider failures without leaking unnecessary payload detail.

### `internal/search` And `internal/retry`

Keep these packages small. Only refactor when it makes a caller or algorithm notably easier to understand. Avoid abstracting tiny helpers further just to match a pattern mechanically.

## File Splitting Guidance

Splitting is optional and must be justified by readability, not symmetry. A split is appropriate when:

- one file contains multiple distinct responsibilities
- the entrypoint is hard to follow because helper noise dominates the file
- testability or discoverability improves materially

Prefer splits such as:

- `service.go` for orchestration
- `service_logging.go` for summary helpers if logging code grows large
- `queries.go` or `scan.go` in repositories when SQL support code overwhelms CRUD methods

Do not split small files or force every package into the same file pattern.

## Verification Strategy

Verification must prove the refactor preserved behavior:

1. Run `gofmt -w ./cmd ./internal`.
2. Run `go test ./...`.
3. Run `go build ./cmd/smem-server`.
4. If a package was heavily reshaped, read its tests and add or adjust focused tests only when the refactor exposes a meaningful coverage gap.

## Success Criteria

The refactor is successful when:

- key entrypoints can be understood quickly from their top-level method bodies
- logs tell the story of major workflow stages
- comments explain design intent rather than syntax
- helper extraction reduces cognitive load instead of scattering logic arbitrarily
- package boundaries remain aligned with the current architecture
- tests and build remain green without behavior drift

## Risks And Guardrails

### Risk: Mechanical Over-Refactor

Blindly extracting tiny helpers can make the code harder to read. Guardrail: only extract helpers when they hide meaningful detail or name an important stage.

### Risk: Logging Noise

Adding logs everywhere can reduce signal. Guardrail: log stage summaries and decisions, not every intermediate value.

### Risk: Accidental Behavior Changes

Reordering code around defaults, retries, or persistence could subtly change behavior. Guardrail: prefer equivalence-preserving rewrites and validate with tests after each logical chunk.

### Risk: Unnecessary File Churn

Large diff volume will make review harder. Guardrail: prioritize high-value files first and avoid touching stable code that already fits the target style.

## Rollout Order

1. Refine style rules in `workflow` packages first.
2. Apply the same patterns to `domain` and `store`.
3. Tidy `llm`, `embedding`, and nearby support code only as needed.
4. Run formatting, tests, and build after substantive edits.

This order keeps the most user-visible flows aligned first and lets lower-level packages follow proven patterns instead of guessing ahead.
