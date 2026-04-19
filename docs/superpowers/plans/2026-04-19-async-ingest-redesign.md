# Async Ingest Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace synchronous memory ingest with an async ingest-job workflow that uses the new prompt schemas, background workers, and a new `ingest_jobs` table.

**Architecture:** Add a durable ingest-job domain and TiDB repository, convert `POST /api/v1/memories` into job submission, and move all normal/smart ingest execution into a background worker that prepares embeddings before committing memory writes and job success state in one transaction.

**Tech Stack:** Go 1.25, Gin, GORM, TiDB/MySQL migrations, existing LLM/embedding providers, existing recall workflow helpers.

---

## File Map

- Create: `server/internal/domain/ingestjob/entity.go`
- Create: `server/internal/domain/ingestjob/enum.go`
- Create: `server/internal/domain/ingestjob/repository.go`
- Create: `server/internal/tidb/ingest_job_repository.go`
- Create: `server/migrations/002_create_ingest_jobs.sql`
- Create: `server/internal/workflow/ingest/job_types.go`
- Create: `server/internal/workflow/ingest/parser.go`
- Create: `server/internal/workflow/ingest/job_worker.go`
- Modify: `server/internal/tidb/model.go`
- Modify: `server/internal/tidb/repository.go`
- Modify: `server/internal/llm/prompt.go`
- Modify: `server/internal/workflow/ingest/service.go`
- Modify: `server/internal/workflow/ingest/ingest_test.go`
- Modify: `server/internal/http/memory_handler.go`
- Modify: `server/internal/http/response.go`
- Modify: `server/internal/http/memory_handler_test.go`
- Modify: `server/internal/app/app.go`
- Test: `server/internal/workflow/ingest/ingest_test.go`
- Test: `server/internal/http/memory_handler_test.go`
- Test: `server/internal/tidb/repository_test.go`

### Task 1: Add Ingest Job Domain And Persistence

**Files:**
- Create: `server/internal/domain/ingestjob/entity.go`
- Create: `server/internal/domain/ingestjob/enum.go`
- Create: `server/internal/domain/ingestjob/repository.go`
- Create: `server/internal/tidb/ingest_job_repository.go`
- Modify: `server/internal/tidb/model.go`
- Modify: `server/internal/tidb/repository.go`
- Create: `server/migrations/002_create_ingest_jobs.sql`
- Test: `server/internal/tidb/repository_test.go`

- [ ] **Step 1: Write the failing repository tests**

Add tests that prove the new repository can submit a job, claim the next executable job, requeue a failed attempt, and mark success with result fields.

- [ ] **Step 2: Run repository tests to verify they fail**

Run: `go test ./internal/tidb -run 'TestIngestJob' -count=1`

Expected: FAIL because the ingest job types/repository and migration-backed schema do not exist yet.

- [ ] **Step 3: Add the minimal ingest-job domain and TiDB persistence**

Implement:
- ingest-job entity and enums for `pending`, `running`, `succeeded`, `failed`
- repository interface for submit, claim, requeue/fail, and success update
- TiDB model and mapping helpers
- SQL migration for `ingest_jobs`

- [ ] **Step 4: Run repository tests to verify they pass**

Run: `go test ./internal/tidb -run 'TestIngestJob' -count=1`

Expected: PASS.

### Task 2: Replace Ingest Prompt Protocol And Worker Execution

**Files:**
- Create: `server/internal/workflow/ingest/job_types.go`
- Create: `server/internal/workflow/ingest/parser.go`
- Create: `server/internal/workflow/ingest/job_worker.go`
- Modify: `server/internal/llm/prompt.go`
- Modify: `server/internal/workflow/ingest/service.go`
- Modify: `server/internal/workflow/ingest/ingest_test.go`

- [ ] **Step 1: Write the failing ingest workflow tests**

Add tests for:
- extraction payload validation and max-5 truncation
- fusion `actions[]` validation
- normal-mode job execution with embedding-before-commit
- smart-mode update/delete/create handling
- retry on embedding failure without memory writes

- [ ] **Step 2: Run ingest workflow tests to verify they fail**

Run: `go test ./internal/workflow/ingest -count=1`

Expected: FAIL because the current ingest service still returns memories synchronously and only understands the old fusion payload schema.

- [ ] **Step 3: Implement the worker-owned ingest flow**

Implement:
- exact extraction and fusion prompt text from `human_doc/design.md`
- strict extraction/fusion JSON parsers
- job submission service replacing synchronous ingest
- poller/worker execution path for normal and smart mode
- embedding preparation before transactional memory writes and job success update

- [ ] **Step 4: Run ingest workflow tests to verify they pass**

Run: `go test ./internal/workflow/ingest -count=1`

Expected: PASS.

### Task 3: Update HTTP/App Wiring For Job Submission

**Files:**
- Modify: `server/internal/http/memory_handler.go`
- Modify: `server/internal/http/response.go`
- Modify: `server/internal/http/memory_handler_test.go`
- Modify: `server/internal/app/app.go`

- [ ] **Step 1: Write the failing HTTP test**

Update the create-memory handler test so `POST /api/v1/memories` expects an ingest job DTO instead of `accepted.items`.

- [ ] **Step 2: Run the HTTP test to verify it fails**

Run: `go test ./internal/http -run TestMemoryHandlerCreateAndGet -count=1`

Expected: FAIL because the handler still returns memory items instead of an ingest job response.

- [ ] **Step 3: Implement the handler and app wiring changes**

Implement:
- ingest-service interface returning a job
- response DTO for ingest jobs
- app wiring for the new ingest job repository and worker startup

- [ ] **Step 4: Run the HTTP test to verify it passes**

Run: `go test ./internal/http -run TestMemoryHandlerCreateAndGet -count=1`

Expected: PASS.

### Task 4: Format And Verify The Full Server

**Files:**
- Modify: any files touched in Tasks 1-3

- [ ] **Step 1: Format the server code**

Run: `gofmt -w ./cmd ./internal`

- [ ] **Step 2: Run the full test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 3: Run the server build**

Run: `go build ./cmd/smem-server`

Expected: PASS.
