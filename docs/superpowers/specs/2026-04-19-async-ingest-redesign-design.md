# Async Ingest Redesign Design

## Goal

Refactor `apps/server` ingest so it follows the `human_doc/design.md` ingest design instead of the current synchronous implementation: `POST /api/v1/memories` should create an ingest job, smart ingest must use the new extraction and fusion prompts, and background workers should execute ingest asynchronously against a new ingest-jobs table.

## Non-Goals

- Adding distributed worker coordination in this iteration.
- Introducing a second child table such as `ingest_job_items`.
- Changing recall ranking behavior beyond reusing existing recall methods during smart ingest.
- Broad refactors outside the ingest, prompt, storage, and HTTP paths needed for this redesign.

## Current Mismatch

The current `apps/server` ingest flow diverges from the approved design in four important ways:

1. `POST /api/v1/memories` executes ingest synchronously and returns `memory` records instead of a queued ingest job.
2. Smart ingest still parses the old fusion payload shape (`decision`, `memory_id`, `content`) rather than the new `actions[]` protocol.
3. There is no durable ingest queue table, so retries and restart-safe progress tracking are not possible.
4. Embedding and memory persistence are coupled to the old in-process queue path rather than the new worker-owned job lifecycle.

This redesign resolves those mismatches in favor of `human_doc/design.md`.

## Scope

This redesign applies to `apps/server` only.

### In Scope

- New `ingest_jobs` persistence model and SQL migration.
- New domain and storage interfaces for ingest jobs.
- HTTP create-memory flow returning an ingest job DTO.
- Background poller-based ingest worker.
- New extraction and fusion prompts copied from `human_doc/design.md`.
- Strict parsing and validation for the new LLM output formats.
- Smart-ingest reconcile execution that supports `create`, `update`, `delete`, and `ignore`.
- Retry handling, idempotency, and restart-safe state transitions.
- Focused tests covering prompt parsing, worker execution, repository behavior, and HTTP response changes.

### Out Of Scope

- Adding APIs to list, inspect, or cancel ingest jobs unless required by existing tests.
- Multi-instance leasing with expiration recovery beyond storing future-compatible fields.
- Reworking unrelated memory read, recall, or delete APIs.

## Architecture

The ingest entrypoint becomes a job-submission flow:

1. `POST /api/v1/memories` validates the request and inserts one `ingest_jobs` row.
2. The HTTP response returns the ingest job, not created memories.
3. A background worker polls executable jobs and processes them one at a time in-process.
4. The worker owns all ingest execution:
   - `normal` mode: build the final durable memory directly.
   - `smart` mode: extract candidate memories, recall relevant memories, reconcile using the new fusion prompt, prepare embeddings for all `create` and `update` results, then commit memory changes and job completion together.

`memories` remains the durable-memory table. `ingest_jobs` becomes the durable execution queue and source of truth for ingest progress.

## API Contract

### Request

`POST /api/v1/memories` keeps the existing create-memory request shape so clients can continue sending `content`, `mode`, `type`, `kinds`, `scope`, and the existing metadata fields.

### Response

`POST /api/v1/memories` returns an ingest job DTO instead of `[]memory`. The minimum response fields are:

- `id`
- `state`
- `mode`
- `execute_count`
- `created_at`
- `updated_at`

The response may also include stable scheduling and error fields such as `next_run_at` and `last_error` if the handler already uses shared DTO patterns that make this practical.

## Ingest Job Model

Introduce a new `ingest_jobs` table and matching domain entity.

### Request Snapshot Fields

- `id`
- `content`
- `type`
- `kinds`
- `scope`
- `mode`
- `metadata`
- `agent_id`
- `session_id`
- `source`

These fields preserve the original ingest request exactly enough that retries do not depend on caller re-submission.

### Execution Fields

- `state`
- `execute_count`
- `next_run_at`
- `locked_at`
- `worker_id`
- `last_error`

These fields support first-iteration polling and preserve a path toward future lease-based workers.

### Result Fields

- `result_memory_ids`
- `result_summary`

These fields let operators inspect what a job changed without deriving it from logs.

### Time Fields

- `created_at`
- `updated_at`

## Job State Machine

### States

- `pending`
- `running`
- `succeeded`
- `failed`

### Transition Rules

1. New jobs start as `pending`.
2. The worker claims one executable job by moving it to `running` and updating `locked_at`, `worker_id`, and `execute_count`.
3. Successful execution moves the job to `succeeded` and records result fields.
4. Failed execution behaves as follows:
   - if `execute_count < 5`, move the job back to `pending`, set `last_error`, and schedule `next_run_at`
   - otherwise move the job to `failed` and preserve `last_error`

The first iteration may run only one in-process worker, but transitions must still be written as if future worker identity matters.

## Worker Execution Model

### Poller

The server starts a single background poller at boot. The poller repeatedly:

1. loads the next executable job with `state = pending` and `next_run_at <= now` or `next_run_at IS NULL`
2. atomically marks it `running`
3. executes the job
4. writes success or retry state

The first iteration does not need distributed leasing. It should still write `worker_id` and `locked_at` so future multi-instance claiming can extend the same table instead of replacing it.

### Idempotency

Retries must not create duplicate durable memories. The main guardrails are:

- `memories.content_hash` remains the exact dedupe key.
- Normal-mode create on hash conflict must increase `store_count` instead of inserting a duplicate row.
- Smart-mode `update` actions may keep the same final content and still count as updates.
- Job retries rerun the worker logic against the current database state instead of assuming the previous attempt partially succeeded.

## Prompt Design

### Extraction Prompt

Replace the current placeholder extraction prompt with the approved prompt text from `human_doc/design.md`.

The worker must parse a single JSON object:

```json
{"memories":[{"content":"...","type":"fact","kinds":["preference"]}]}
```

Rules:

- Accept at most 5 candidate memories.
- Ignore unknown extra fields.
- Reject malformed JSON.
- Reject invalid `type` values outside `fact`, `episodic`, `procedural`, `""`.
- Filter invalid `kinds` values to the allowed fixed enum set from the design doc.
- Preserve candidate order.

If extraction returns zero memories, smart ingest completes successfully with no memory writes.

### Fusion Prompt

Replace the current placeholder fusion prompt with the approved prompt text from `human_doc/design.md`.

The worker must parse a single JSON object:

```json
{"actions":[...]}
```

Each action must satisfy the protocol:

- `target` is `candidate` or `memory`
- candidate actions are only `ignore` or `create`
- memory actions are only `update`, `delete`, or `ignore`
- `create` and `update` must include `memory.content`
- `delete` and `ignore` must not include `memory`
- candidate ids must appear exactly once in input order
- recalled memory ids must appear exactly once in first-seen order

Protocol violations are hard failures for the job attempt. The worker must not silently fall back to the old fusion schema.

## Normal-Mode Execution

For `mode = normal`:

1. Normalize the request into one final memory candidate.
2. Generate the embedding before opening the write transaction.
3. Open one transaction.
4. Insert or dedupe-update the durable memory using `content_hash`.
5. Mark the ingest job `succeeded` in the same transaction.
6. Commit.

If embedding fails, do not open the write transaction. Update the job for retry instead.

## Smart-Mode Execution

For `mode = smart`:

1. Run extraction with the new extraction prompt.
2. Convert each extracted candidate into a memory-shaped candidate using the original request scope and metadata.
3. For each candidate, reuse existing recall methods to fetch up to 3 relevant memories.
4. Build the fusion prompt input from all candidates plus the union of recalled memories.
5. Parse and validate the returned `actions[]`.
6. Materialize the final write set:
   - `candidate/create` becomes a new durable memory
   - `memory/update` becomes an update to an existing durable memory
   - `memory/delete` deletes the existing durable memory
   - `ignore` actions produce no write
7. Generate embeddings for every final `create` and `update` memory before opening the write transaction.
8. Open one transaction and apply all memory writes plus job completion together.
9. Commit.

This keeps reconcile execution atomic at the database level while still honoring the requirement that embedding runs before the memory-commit transaction.

## Recall Usage During Smart Ingest

The redesign should reuse existing recall-related methods rather than invent a second search flow. Each extracted candidate may recall up to 3 memories, matching the approved design. The smart-ingest code may add small helpers to adapt existing recall outputs into the fusion-prompt input shape, but it should not fork the ranking logic or add prompt-specific search rules outside current recall behavior.

## Transaction Boundaries

Two constraints must both hold:

1. Embedding runs outside the memory write transaction.
2. Memory writes and final job success state commit in the same transaction.

Therefore:

- extraction, recall, fusion, and embedding happen before the transaction
- the transaction contains only durable memory create/update/delete work and the terminal `succeeded` job update
- failed attempts update the job outside that memory transaction because no memory writes should commit on failure

This design avoids partially written memories with missing embeddings while still giving job retries full control over persistence.

## Package-Level Changes

### `internal/workflow/ingest`

- Change the top-level service from synchronous memory creation to ingest-job submission.
- Introduce focused helpers for extraction parsing, fusion parsing, worker execution, and result summarization.
- Remove the old direct `Worker.Queue(memory.Memory)` assumption and replace it with job-oriented execution.

### `internal/llm`

- Replace both ingest prompt builders with the approved prompt text and input formatting.
- Keep shared retry behavior unchanged.

### `internal/domain`

- Add an ingest-job entity, enums, and repository interface.
- Keep durable-memory rules in the memory domain package; job orchestration should not move memory CRUD rules into HTTP handlers.

### `internal/store/tidb`

- Add `ingest_jobs` model, mapping helpers, repository implementation, and migration.
- Support job submission, claiming, retry updates, and success updates.
- Preserve `memories` persistence as the source of truth for durable memories.

### `internal/transport/http`

- Update the create-memory handler and DTOs to return the ingest job response shape.
- Preserve request validation rules unless they conflict with the new design.

## Error Handling

### Hard Failures For A Job Attempt

- invalid extraction JSON
- invalid fusion JSON
- fusion protocol mismatch
- embedding provider failure
- memory transaction failure

These failures increment `execute_count` and drive retry or terminal failure.

### Non-Failure Outcomes

- extraction returns zero candidates
- smart ingest produces only `ignore` actions
- normal ingest dedupes into `store_count` update instead of inserting a row

These are successful job outcomes and should mark the job `succeeded`.

## Observability

Add concise logs at stage boundaries:

- job submitted
- job claimed
- extraction result count
- recall result counts per candidate
- fusion action summary
- transaction result summary
- retry scheduled or terminal failure

Logs should summarize ids and counts, not dump full request payloads by default.

## Testing Strategy

### Parser Tests

- extraction payload success, empty result, invalid JSON, invalid type, invalid kinds filtering, max-5 truncation
- fusion payload success, missing action coverage, illegal action types, illegal `memory` presence, stable ordering validation

### Worker Tests

- normal job creates one memory and succeeds
- normal job dedupes by `content_hash` and increments `store_count`
- smart job creates new memories from valid `create` actions
- smart job updates existing memories when fusion says `update`, including unchanged-content updates
- smart job deletes contradicted memories when fusion says `delete`
- smart job retries on embedding failure without committing memory writes
- smart job succeeds with zero writes when extraction returns no candidates or all actions are `ignore`

### Repository Tests

- submit job
- claim next executable job
- move running job back to pending with retry metadata
- mark terminal failure after the fifth attempt
- mark success with result fields

### HTTP Tests

- `POST /api/v1/memories` returns an ingest job DTO instead of `memory[]`
- request validation still works for normal and smart mode submissions

## Success Criteria

The redesign is successful when:

- `POST /api/v1/memories` creates and returns an ingest job
- ingest execution survives process restart because job state is stored durably
- smart ingest uses the new extraction and fusion prompts exactly
- fusion parsing enforces the new `actions[]` protocol with no fallback to the old schema
- embeddings are generated before the memory write transaction
- memory writes and job success state commit together
- retries stop after 5 failed attempts
- tests cover the new parsing, worker, repository, and HTTP behavior

## Risks And Guardrails

### Risk: Silent Prompt Drift

If prompt builders paraphrase the approved text, downstream behavior will drift. Guardrail: copy the approved prompt text exactly from `human_doc/design.md` into the implementation source.

### Risk: Partial State On Retry

If memory writes commit before job success state or before embeddings are ready, retries may duplicate or corrupt results. Guardrail: compute embeddings first and commit memory writes plus final job success in one transaction.

### Risk: Over-Building The Queue

A first-pass single worker can become needlessly complex if it fully designs for distributed execution. Guardrail: store future-facing fields now but keep the first worker model single-process and readable.

### Risk: Smart-Mode Logic Becomes Opaque

Extraction, recall, fusion, embedding, and transaction code can easily collapse into one large function. Guardrail: keep the top-level worker flow linear and extract named helpers for each stage.
