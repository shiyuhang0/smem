# AGENTS Guide
This file is for coding agents working in `smem`.

## Repository Status
- Active implementation is currently in `apps/server`.
- `apps/plugin-openclaw` and `apps/dashboard` are placeholders.
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
- Run server commands from `apps/server`.
- Run repo-level inspections from the repository root.
- Do not assume root-level Go commands work; the Go module lives in `apps/server`.

## Build Commands
From `apps/server`:

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
go test ./internal/workflow/ingest
go test ./internal/workflow/recall
go test ./internal/store/tidb
go test ./internal/transport/http
```

Run one specific test:

```bash
go test ./internal/store/tidb -run TestTiDBCloudConnection -v
go test ./internal/workflow/ingest -run TestSmartCreateUsesFusionDecisionToUpdateExistingMemory -v
go test ./internal/workflow/recall -run TestRecallUsesRewriteTypeAndKindsToBoostMatches -v
go test ./internal/transport/http -run TestMemoryHandlerCreateAndGet -v
```

Disable test caching when debugging:

```bash
go test ./internal/transport/http -count=1 -v
```

Run the TiDB Cloud integration test:

```bash
SMEM_INTEGRATION_TIDB=1 DB_DSN='...' DB_TLS_SERVER_NAME='...' go test ./internal/store/tidb -run TestTiDBCloudConnection -v
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
- `internal/config`: environment-based config loading
- `internal/domain/memory`: core types, validation, service logic
- `internal/transport/http`: HTTP handlers and DTOs
- `internal/store/tidb`: GORM models, repository, TLS DSN prep, migrations
- `internal/retry`: shared retry policy for LLM and embedding calls
- `internal/llm`: LLM abstraction and OpenAI-compatible implementation
- `internal/embedding`: embedding abstraction and OpenAI-compatible implementation
- `internal/workflow/ingest`: normal/smart ingest orchestration
- `internal/workflow/recall`: recall orchestration
- `internal/search`: fusion and rerank helpers

Follow the existing layering. Do not put HTTP concerns into domain packages.

## Code Style
### General
- Prefer small, focused files.
- Prefer minimal changes over architectural churn.
- Keep logic close to where it is used unless reuse is clear.
- Match existing naming and package boundaries before introducing new abstractions.

### Formatting
- Always use `gofmt`.
- Use tabs as Go tooling expects.
- Keep lines readable; avoid dense one-liners when the logic is non-trivial.

### Imports
- Let `gofmt` manage import grouping.
- Standard library imports first, third-party imports next, local module imports last.
- Prefer import aliases only when needed to resolve collisions or clarify intent.
- Existing aliases like `stdhttp` and `searchfusion` are acceptable; use them sparingly.

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
- `apps/server/api/openapi.yaml`

Keep docs aligned with real behavior, not intended behavior.

## Before Finishing
From `apps/server`, run:

```bash
gofmt -w ./cmd ./internal
go test ./...
go build ./cmd/smem-server
```

If you changed DB connectivity or TiDB TLS behavior and credentials are available, also run:

```bash
SMEM_INTEGRATION_TIDB=1 DB_DSN='...' DB_TLS_SERVER_NAME='...' go test ./internal/store/tidb -run TestTiDBCloudConnection -v
```

Do not claim completion without fresh command output.
