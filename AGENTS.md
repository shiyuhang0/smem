# AGENTS Guide
This file is for coding agents working in `smem`.

## Repository Status
- Active implementation is currently in `server/`.
- `apps/plugin-openclaw` and `apps/dashboard` are placeholders (not present in tree yet).
- There is no `Makefile`, no root `package.json`, and no repo-wide task runner.
- There is no `.cursorrules`, no `.cursor/rules/`, and no `.github/copilot-instructions.md` in this repository.

## Tech Stack
- Go 1.25
- Gin
- GORM
- TiDB
- SQLite in-memory for repository tests
- OpenAI-compatible HTTP APIs for LLM and embeddings

## Working Directory
- Run server commands from `server/`.
- Run repo-level inspections from the repository root.
- Do not assume root-level Go commands work; the Go module lives in `server/` (module path `smem/apps/server`).

## Build Commands
From `server/`:

```bash
go build ./cmd/smem-server
go build ./...
go run ./cmd/smem-server
```

## Test Commands
Run all tests:

```bash
go test ./...
```

Run one package:

```bash
go test ./internal/domain/ingest
go test ./internal/domain/recall
go test ./internal/store
go test ./internal/handler
```

Run one specific test:

```bash
go test ./internal/store -run TestTiDBCloudConnection -v
go test ./internal/domain/ingest -run TestSmartCreateUsesFusionDecisionToUpdateExistingMemory -v
go test ./internal/domain/recall -run TestRecallUsesRewriteTypeAndKindsToBoostMatches -v
go test ./internal/handler -run TestMemoryHandlerCreateAndGet -v
```

Disable test caching when debugging:

```bash
go test ./internal/handler -count=1 -v
```

Run the TiDB Cloud integration test:

```bash
SMEM_INTEGRATION_TIDB=1 DB_DSN='...' DB_TLS_SERVER_NAME='...' go test ./internal/tidb -run TestTiDBCloudConnection -v
```

## Formatting / Linting
There is no dedicated linter config yet. Use Go formatting and tests as the baseline:

```bash
gofmt -w ./cmd ./internal
go test ./...
```

If you touch imports, run `gofmt`; do not hand-format import blocks.

## Dependency Management
When dependencies change:

```bash
go mod tidy
```

Do not edit `go.mod` or `go.sum` manually unless absolutely necessary.

## Architecture Overview
- `cmd/smem-server`: binary entrypoint
- `internal/app`: app wiring and router setup
- `internal/config`: environment-based config loading and std logger helper (`NewLogger`)
- `internal/domain/memory`: core types, validation, service logic
- `internal/domain/ingestjob`: ingest job types and repository interface
- `internal/handler`: HTTP handlers and DTOs
- `internal/store`: GORM models, repository, TLS DSN prep, migrations
- `internal/ai/llm`: LLM abstraction and OpenAI-compatible implementation
- `internal/ai/embedding`: embedding abstraction and OpenAI/Ollama implementations
- `internal/ai/retry`: shared retry policy for LLM and embedding calls
- `internal/domain/ingest`: normal/smart ingest orchestration
- `internal/domain/recall`: recall orchestration
- `internal/domain/recall/scoring`: RRF fusion, rerank scoring, softmax

Follow the existing layering. Do not put HTTP concerns into domain packages.

## Code Style
### General
- Prefer small, focused files.
- Prefer minimal changes over architectural churn.
- Keep logic close to where it is used unless reuse is clear.
- Match existing naming and package boundaries before introducing new abstractions.

### Recall-Style Flows
- For service and workflow entrypoints, prefer the style demonstrated by `server/internal/domain/recall/service.go` in `Recall`.
- Keep top-level logic easy to read: normalize input first, keep the main path linear, and avoid burying the flow in dense branches.
- Add comments only at important decision points, trade-offs, or future extension points.
- Keep key stages observable with concise logs so the recall/ingest/search path can be diagnosed from runtime output.
- Extract meaningful helper methods so the top-level method stays clear and stage-oriented.

### Formatting
- Always use `gofmt`.
- Use tabs as Go tooling expects.
- Keep lines readable; avoid dense one-liners when the logic is non-trivial.

### Imports
- Let `gofmt` manage import grouping.
- Standard library imports first, third-party imports next, local module imports last.
- Prefer import aliases only when needed to resolve collisions or clarify intent.
- Existing aliases like `stdhttp` are acceptable; use them sparingly.

### Naming
- Exported names: `PascalCase`.
- Unexported names: `camelCase`.
- Package names should stay short and lowercase.
- Avoid stutter: prefer `memory.Service`, not `memory.MemoryService`.
- Use descriptive method names: `Create`, `Recall`, `PrepareDSN`, `GenerateText`, `Embed`.

### Types
- Use typed enums for domain concepts: `memory.Type`, `memory.Scope`, `memory.State`, `memory.Mode`.
- Prefer concrete structs over `map[string]any` unless the shape is truly dynamic.
- Use pointers in request/update structs only when omitted vs zero value matters.
- Keep JSON DTOs separate from domain entities.

### Error Handling
- Return errors; do not panic in production paths.
- Use `fmt.Errorf` for contextual errors.
- Use sentinel errors when callers need branching behavior, e.g. `memory.ErrNotFound`.
- Map domain/storage errors to HTTP status codes in one place.
- For external calls, preserve retry behavior and only retry retryable failures.

### Context, Time, IDs
- Pass `context.Context` through service, repository, LLM, and embedding boundaries.
- In HTTP handlers, use `c.Request.Context()` instead of passing `*gin.Context` downward.
- Use UTC timestamps.
- Prefer injected clock/ID functions when tests need deterministic behavior.

### Testing Style
- Add tests before behavior changes when practical.
- Keep tests package-local when they validate internals.
- Prefer table-driven tests for validation and branch-heavy logic.
- Use `require` for hard assertions and setup checks.
- Repository tests may use in-memory SQLite; integration tests must be gated by env vars.

## Repository-Specific Conventions
- `smart ingest` must degrade cleanly to `normal ingest` when LLM extraction fails.
- LLM and embedding calls must keep the shared retry policy: 3 attempts, retry only retryable failures.
- `creating` memories are not searchable or recallable.
- TiDB TLS support is configured through `DB_DSN` and `DB_TLS_SERVER_NAME`.

## Documentation Expectations
If behavior changes, update:
- `README.md`
- `docs/getting-started.md`
- `docs/server/*.md`
- `docs/deploy/*.md`
- `server/api/openapi.yaml`

Keep docs aligned with real behavior, not intended behavior.

## Before Finishing
From `server/`, run:

```bash
gofmt -w ./cmd ./internal
go test ./...
go build ./cmd/smem-server
```

If you changed DB connectivity or TiDB TLS behavior and credentials are available, also run:

```bash
SMEM_INTEGRATION_TIDB=1 DB_DSN='...' DB_TLS_SERVER_NAME='...' go test ./internal/store -run TestTiDBCloudConnection -v
```

Do not claim completion without fresh command output.
