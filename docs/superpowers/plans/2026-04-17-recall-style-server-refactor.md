# Recall-Style Server Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `server/` so the main Go workflows read like `Recall`: linear top-level flows, clear helper extraction, focused comments, and stable stage-level observability, without changing behavior.

**Architecture:** Keep the current package boundaries intact and treat this as a behavior-preserving refactor. Start from `workflow` entrypoints, then align `domain`, `store`, and provider code to the same orchestration shape, only touching support packages when they materially improve the readability of the higher-level flows.

**Tech Stack:** Go 1.25, Gin, GORM, TiDB, SQLite in-memory tests, OpenAI-compatible HTTP APIs

---

## File Structure

### Files To Modify

- `server/internal/workflow/recall/service.go`: keep `Recall` as the style source, but make helper boundaries and summaries support the same top-level readability.
- `server/internal/workflow/recall/service_test.go`: lock behavior around defaults and ranking flow before refactoring.
- `server/internal/workflow/ingest/service.go`: restructure normal create, smart create, and fusion decisions into stage-based orchestration with logs.
- `server/internal/workflow/ingest/ingest_test.go`: lock smart-create fallback and decision behavior during refactor.
- `server/internal/domain/memory/service.go`: split create/update/list into normalize, build, persist stages.
- `server/internal/domain/memory/service_test.go`: lock list normalization and create dedup behavior.
- `server/internal/tidb/repository.go`: keep CRUD entrypoints thin and move repeated query/scan details behind helpers.
- `server/internal/tidb/repository_test.go`: lock search/list/query helper behavior.
- `server/internal/llm/openai.go`: make request building, retry execution, and response decoding explicit stages.
- `server/internal/llm/openai_test.go`: lock empty-choice handling while preserving retry behavior.
- `server/internal/embedding/openai.go`: same staged refactor for OpenAI embeddings.
- `server/internal/embedding/ollama.go`: same staged refactor for Ollama embeddings.
- `server/internal/embedding/openai_test.go`: lock empty-data handling for OpenAI embedding responses.

### Files To Create

- `server/internal/embedding/ollama_test.go`: lock `embeddings` vs `embedding` response handling before the Ollama provider refactor.
- `server/internal/tidb/queries.go`: hold reusable query normalization and SQL/query-builder helpers if `repository.go` becomes hard to scan.
- `server/internal/tidb/scan.go`: hold row-scan helpers if moving them out makes CRUD/search methods shorter and clearer.

Only create `queries.go` and `scan.go` if the refactor actually moves concrete logic there during implementation. If `repository.go` stays readable after helper extraction, keep the helpers in `repository.go` and do not create the files.

### Files Likely Unchanged

- `server/internal/search/*`: keep as-is unless a tiny rename or comment is needed to support readability in callers.
- `server/internal/retry/*`: preserve semantics; only touch if provider refactors reveal a small clarity improvement.
- `server/internal/http/*`: leave untouched unless a service signature cleanup forces a trivial callsite adjustment.

## Task 1: Refactor Recall Flow Without Changing Ranking Behavior

**Files:**
- Modify: `server/internal/workflow/recall/service.go`
- Test: `server/internal/workflow/recall/service_test.go`

- [ ] **Step 1: Add a behavior-lock test for Recall defaults**

```go
func TestRecallUsesDefaultTopKWhenInputTopKIsZero(t *testing.T) {
	now := time.Now().UTC()
	repo := &recallRepo{
		vectorCandidates: []memory.RecallCandidate{
			{Memory: memory.Memory{ID: "a", Content: "alpha", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}, VectorDistance: floatPtr(0.1)},
			{Memory: memory.Memory{ID: "b", Content: "beta", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}, VectorDistance: floatPtr(0.2)},
			{Memory: memory.Memory{ID: "c", Content: "gamma", State: memory.StateActive, StoreCount: 1, CreatedAt: now, UpdatedAt: now}, VectorDistance: floatPtr(0.3)},
		},
	}
	svc := NewService(repo, fakeEmbedder{}, fakeLLMProvider{})

	results, err := svc.Recall(context.Background(), memory.RecallInput{Content: "remember this"})
	require.NoError(t, err)
	require.Len(t, results, 2)
}
```

- [ ] **Step 2: Run the targeted recall tests and confirm the new test fails before the refactor**

Run: `go test ./internal/workflow/recall -run 'TestRecallUsesDefaultTopKWhenInputTopKIsZero|TestRecallUsesVectorDistanceDuringRerank|TestRecallUsesRRFToMergeVectorAndFullTextCandidates' -count=1 -v`

Expected: the new default-value test fails first if `Recall` does not yet guarantee the exact defaulting path you want to preserve.

- [ ] **Step 3: Refactor `Recall` into staged helpers while keeping its readable top-level flow**

```go
func (s *Service) Recall(ctx context.Context, input memory.RecallInput) ([]memory.RecallResult, error) {
	input = normalizeRecallInput(input)

	query := buildRecallQuery(input)
	candidates, err := s.loadRecallCandidates(ctx, query, input.TopK)
	if err != nil {
		return nil, err
	}

	results := s.rankRecallCandidates(candidates, input)
	if len(results) == 0 {
		return nil, nil
	}

	return s.finalizeRecallResults(results, input), nil
}

func normalizeRecallInput(input memory.RecallInput) memory.RecallInput {
	if input.TopK == 0 {
		input.TopK = 2
	}
	if input.Temperature == 0 {
		input.Temperature = 1
	}
	return input
}
```

- [ ] **Step 4: Keep comments and logs only at stage boundaries**

```go
func (s *Service) loadRecallCandidates(ctx context.Context, query string, topK int) ([]memory.RecallCandidate, error) {
	vectorCandidates, err := s.searchVectorCandidates(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	recallLogger.Printf("vector_search_results=%v", summarizeRecallCandidates(vectorCandidates))

	fullTextCandidates, err := s.searchFullTextCandidates(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	recallLogger.Printf("full_search_results=%v", summarizeRecallCandidates(fullTextCandidates))

	rrfCandidates := s.rrfCandidates(vectorCandidates, fullTextCandidates, topK)
	recallLogger.Printf("rrf_candidates=%v", summarizeRecallCandidates(rrfCandidates))
	return rrfCandidates, nil
}
```

- [ ] **Step 5: Run the full recall package tests**

Run: `go test ./internal/workflow/recall -count=1 -v`

Expected: PASS for all recall tests, including the new default-value test.

- [ ] **Step 6: Commit the recall refactor**

```bash
git add server/internal/workflow/recall/service.go server/internal/workflow/recall/service_test.go
git commit -m "refactor: streamline recall workflow"
```

## Task 2: Make Ingest Read Like A Stage Pipeline

**Files:**
- Modify: `server/internal/workflow/ingest/service.go`
- Test: `server/internal/workflow/ingest/ingest_test.go`

- [ ] **Step 1: Add a behavior-lock test for incomplete update decisions**

```go
func TestSmartCreateTreatsUpdateWithoutMemoryIDAsCreate(t *testing.T) {
	repo := newRepo()
	memSvc := memory.NewService(repo, func() string { return "memory-1" })
	worker := &fakeWorker{}
	svc := NewService(memSvc, repo, worker, &sequenceLLM{responses: []fakeResponse{
		{text: `{"memories":[{"content":"user prefers neovim","type":"fact","kinds":["preference"]}]}`},
		{text: `{"decision":"update","content":"user prefers neovim"}`},
	}})

	items, err := svc.Create(context.Background(), memory.CreateInput{Content: "I prefer neovim", Mode: memory.ModeSmart})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "memory-1", items[0].ID)
}
```

- [ ] **Step 2: Run the targeted ingest tests**

Run: `go test ./internal/workflow/ingest -run 'TestSmartCreateTreatsUpdateWithoutMemoryIDAsCreate|TestSmartCreateFallsBackToNormalWhenExtractionFails|TestSmartCreateUsesFusionDecisionToUpdateExistingMemory' -count=1 -v`

Expected: the new test fails first if the current fallback path is not explicitly protected before the refactor.

- [ ] **Step 3: Reshape `Create` into explicit normal-path and smart-path stages**

```go
func (s *Service) Create(ctx context.Context, input memory.CreateInput) ([]memory.Memory, error) {
	if input.Mode != memory.ModeSmart {
		return s.createNormal(ctx, input)
	}

	items, err := s.createSmart(ctx, input)
	if err == nil {
		return items, nil
	}

	ingestLogger.Printf("smart_create_fallback err=%v", err)
	return s.createNormal(ctx, input)
}

func (s *Service) createNormal(ctx context.Context, input memory.CreateInput) ([]memory.Memory, error) {
	item, err := s.memoryService.Create(ctx, input)
	if err != nil {
		return nil, err
	}
	if err := s.queueCreatedMemory(ctx, item); err != nil {
		return nil, err
	}
	return []memory.Memory{item}, nil
}
```

- [ ] **Step 4: Split smart-ingest internals into named stages and add concise decision logs**

```go
func (s *Service) createSmart(ctx context.Context, input memory.CreateInput) ([]memory.Memory, error) {
	candidates, err := s.extractSmartCandidates(ctx, input)
	if err != nil {
		return nil, err
	}

	items := make([]memory.Memory, 0, len(candidates))
	for _, candidate := range candidates {
		resolved, err := s.resolveSmartCandidate(ctx, candidate)
		if err != nil {
			return nil, err
		}
		if resolved == nil {
			ingestLogger.Printf("fusion_decision=ignore content=%q", candidate.Content)
			continue
		}
		if err := s.queueCreatedMemory(ctx, *resolved); err != nil {
			return nil, err
		}
		items = append(items, *resolved)
	}
	return items, nil
}
```

- [ ] **Step 5: Run the full ingest package tests**

Run: `go test ./internal/workflow/ingest -count=1 -v`

Expected: PASS for normal create, smart fallback, update decision, ignore decision, and the new incomplete-update test.

- [ ] **Step 6: Commit the ingest refactor**

```bash
git add server/internal/workflow/ingest/service.go server/internal/workflow/ingest/ingest_test.go
git commit -m "refactor: clarify ingest workflow stages"
```

## Task 3: Align Memory Service Methods To The Same Orchestration Style

**Files:**
- Modify: `server/internal/domain/memory/service.go`
- Test: `server/internal/domain/memory/service_test.go`

- [ ] **Step 1: Add a behavior-lock test for list normalization**

```go
func TestServiceListNormalizesPaginationDefaults(t *testing.T) {
	repo := newFakeRepository()
	svc := NewService(repo, func() string { return "memory-1" })

	_, _, err := svc.List(context.Background(), ListInput{})
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run the targeted memory service tests**

Run: `go test ./internal/domain/memory -run 'TestServiceListNormalizesPaginationDefaults|TestServiceCreateDeduplicatesByContentHash' -count=1 -v`

Expected: the new list-normalization test fails first if pagination defaults are not yet covered tightly enough.

- [ ] **Step 3: Extract create/list/update helper stages without changing domain rules**

```go
func (s *Service) Create(ctx context.Context, input CreateInput) (Memory, error) {
	if err := input.Validate(); err != nil {
		return Memory{}, err
	}

	item := s.buildMemory(input)
	return s.createOrIncrementExisting(ctx, item)
}

func (s *Service) List(ctx context.Context, input ListInput) ([]Memory, int64, error) {
	input = normalizeListInput(input)
	return s.repo.List(ctx, input)
}
```

- [ ] **Step 4: Make update flow name its intent explicitly**

```go
func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (Memory, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Memory{}, err
	}

	updated, err := s.applyUpdateInput(item, input)
	if err != nil {
		return Memory{}, err
	}

	updated.UpdatedAt = s.now().UTC()
	return s.repo.Update(ctx, updated)
}
```

- [ ] **Step 5: Run the full memory package tests**

Run: `go test ./internal/domain/memory -count=1 -v`

Expected: PASS for create dedup, validation, and the new list-normalization test.

- [ ] **Step 6: Commit the memory service refactor**

```bash
git add server/internal/domain/memory/service.go server/internal/domain/memory/service_test.go
git commit -m "refactor: simplify memory service flows"
```

## Task 4: Thin Out TiDB Repository Entrypoints

**Files:**
- Modify: `server/internal/tidb/repository.go`
- Modify or Create: `server/internal/tidb/queries.go`
- Modify or Create: `server/internal/tidb/scan.go`
- Test: `server/internal/tidb/repository_test.go`

- [ ] **Step 1: Add a behavior-lock test for search normalization**

```go
func TestSearchUsesDefaultLimitAndOnlyReturnsActiveRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&MemoryModel{}))

	repo := NewRepository(db)
	now := time.Unix(100, 0).UTC()
	require.NoError(t, db.Create(&MemoryModel{
		ID:          "active-1",
		Content:     "remember vim",
		ContentHash: "hash-1",
		Scope:       string(memory.ScopeUser),
		State:       string(memory.StateActive),
		Kinds:       StringSlice{},
		Metadata:    JSONMap{},
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error)
	require.NoError(t, db.Create(&MemoryModel{
		ID:          "archived-1",
		Content:     "remember vim",
		ContentHash: "hash-2",
		Scope:       string(memory.ScopeUser),
		State:       string(memory.StateArchived),
		Kinds:       StringSlice{},
		Metadata:    JSONMap{},
		Version:     1,
		StoreCount:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error)

	items, err := repo.Search(context.Background(), "vim", 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "active-1", items[0].ID)
}
```

- [ ] **Step 2: Run the targeted repository tests**

Run: `go test ./internal/tidb -run 'TestSearchUsesDefaultLimitAndOnlyReturnsActiveRows|TestRepositoryCRUDAndList|TestScanRecallCandidateRowsPreservesDistanceAndFullTextScore|TestFullTextQueryLiteralEscapesSpecialCharacters' -count=1 -v`

Expected: the new search-normalization test fails first if the current behavior is not fully protected before moving helpers.

- [ ] **Step 3: Keep repository entrypoints linear and move repeated detail into helpers**

```go
func (r *Repository) Search(ctx context.Context, query string, limit int) ([]memory.Memory, error) {
	query, limit = normalizeSearchInput(query, limit)
	if query == "" {
		return nil, nil
	}

	models, err := r.runSearchQuery(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	return toDomainMemories(models), nil
}
```

- [ ] **Step 4: Move scan/query helpers only if they make CRUD/search methods shorter**

```go
func normalizeSearchInput(query string, limit int) (string, int) {
	query = strings.TrimSpace(query)
	if limit <= 0 {
		limit = 10
	}
	return query, limit
}

func toDomainMemories(models []MemoryModel) []memory.Memory {
	out := make([]memory.Memory, 0, len(models))
	for _, model := range models {
		out = append(out, model.toDomain())
	}
	return out
}
```

- [ ] **Step 5: Run the full TiDB package tests**

Run: `go test ./internal/tidb -count=1 -v`

Expected: PASS for CRUD, list, row scan, literal escaping, and the new search normalization test.

- [ ] **Step 6: Commit the repository refactor**

```bash
git add server/internal/tidb/repository.go server/internal/tidb/repository_test.go server/internal/tidb/queries.go server/internal/tidb/scan.go
git commit -m "refactor: simplify tidb repository flows"
```

## Task 5: Refactor Provider Clients Into Request / Execute / Decode Stages

**Files:**
- Modify: `server/internal/llm/openai.go`
- Modify: `server/internal/llm/openai_test.go`
- Modify: `server/internal/embedding/openai.go`
- Modify: `server/internal/embedding/openai_test.go`
- Modify: `server/internal/embedding/ollama.go`
- Create: `server/internal/embedding/ollama_test.go`

- [ ] **Step 1: Add behavior-lock tests for empty provider payloads**

```go
func TestOpenAIProviderReturnsErrorWhenChoicesAreMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{}})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(Config{
		BaseURL:    server.URL,
		APIKey:     "test",
		Model:      "gpt-4.1-mini",
		HTTPClient: server.Client(),
		Retry:      retry.Policy{MaxAttempts: 1, Backoff: func(int) {}, IsRetryable: retry.DefaultRetryable},
	})

	_, err := provider.GenerateText(context.Background(), []Message{{Role: "user", Content: "hello"}})
	require.EqualError(t, err, "openai response has no choices")
}
```

```go
func TestOpenAIEmbeddingProviderReturnsErrorWhenDataMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(Config{
		BaseURL:    server.URL,
		APIKey:     "test",
		Model:      "text-embedding-3-small",
		HTTPClient: server.Client(),
		Retry:      retry.Policy{MaxAttempts: 1, Backoff: func(int) {}, IsRetryable: retry.DefaultRetryable},
	})

	_, err := provider.Embed(context.Background(), "hello")
	require.EqualError(t, err, "embedding response has no data")
}
```

```go
func TestOllamaProviderUsesEmbeddingsArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embeddings": [][]float64{{0.1, 0.2, 0.3}},
		})
	}))
	defer server.Close()

	provider := NewOllamaProvider(Config{
		BaseURL:    server.URL,
		Model:      "nomic-embed-text",
		HTTPClient: server.Client(),
		Retry:      retry.Policy{MaxAttempts: 1, Backoff: func(int) {}, IsRetryable: retry.DefaultRetryable},
	})

	vector, err := provider.Embed(context.Background(), "hello")
	require.NoError(t, err)
	require.Equal(t, []float32{0.1, 0.2, 0.3}, vector)
}
```

- [ ] **Step 2: Run the targeted provider tests**

Run: `go test ./internal/llm ./internal/embedding -run 'TestOpenAIProviderReturnsErrorWhenChoicesAreMissing|TestOpenAIProviderRetriesAndReturnsMessage|TestOpenAIEmbeddingProviderReturnsErrorWhenDataMissing|TestOpenAIProviderRetriesAndReturnsVector|TestOllamaProviderUsesEmbeddingsArray' -count=1 -v`

Expected: the new empty-payload tests fail first if the current provider code does not yet preserve those error paths explicitly.

- [ ] **Step 3: Refactor the LLM provider into explicit request, execute, and decode helpers**

```go
func (p *OpenAIProvider) GenerateText(ctx context.Context, messages []Message) (string, error) {
	body, err := p.marshalChatRequest(messages)
	if err != nil {
		return "", err
	}

	payload, err := p.doChatCompletion(ctx, body)
	if err != nil {
		return "", err
	}

	return decodeChatCompletion(payload)
}
```

- [ ] **Step 4: Apply the same staged structure to OpenAI embedding and Ollama embedding providers**

```go
func (p *OpenAIProvider) Embed(ctx context.Context, input string) ([]float32, error) {
	body, err := p.marshalEmbeddingRequest(input)
	if err != nil {
		return nil, err
	}

	payload, err := p.doEmbeddingRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	return decodeOpenAIEmbedding(payload)
}
```

```go
func (p *OllamaProvider) Embed(ctx context.Context, input string) ([]float32, error) {
	body, err := p.marshalEmbeddingRequest(input)
	if err != nil {
		return nil, err
	}

	payload, err := p.doEmbeddingRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	return decodeOllamaEmbedding(payload)
}
```

- [ ] **Step 5: Run provider package tests after the refactor**

Run: `go test ./internal/llm ./internal/embedding -count=1 -v`

Expected: PASS for retry behavior and the new empty-payload / embeddings-array tests.

- [ ] **Step 6: Commit the provider refactor**

```bash
git add server/internal/llm/openai.go server/internal/llm/openai_test.go server/internal/embedding/openai.go server/internal/embedding/openai_test.go server/internal/embedding/ollama.go server/internal/embedding/ollama_test.go
git commit -m "refactor: stage provider request handling"
```

## Task 6: Format, Verify, And Do The Smallest Necessary Cleanup

**Files:**
- Modify only if needed by compilation or readability fallout from Tasks 1-5

- [ ] **Step 1: Run formatting across the server module**

Run: `gofmt -w ./cmd ./internal`

Expected: no output, files rewritten in place if needed.

- [ ] **Step 2: Run the full test suite**

Run: `go test ./...`

Expected: PASS across workflow, domain, store, provider, and transport packages.

- [ ] **Step 3: Build the server binary**

Run: `go build ./cmd/smem-server`

Expected: build succeeds with no compile errors.

- [ ] **Step 4: If Tasks 1-5 forced a tiny support-package cleanup, commit that cleanup with the final verification**

```bash
git add server/internal
git commit -m "refactor: align server code with recall style"
```

## Self-Review

- Spec coverage check:
  - `workflow/recall` is covered by Task 1.
  - `workflow/ingest` is covered by Task 2.
  - `domain/memory` is covered by Task 3.
  - `store/tidb` is covered by Task 4.
  - `llm` and `embedding` are covered by Task 5.
  - formatting, tests, and build verification are covered by Task 6.
- Placeholder scan:
  - No `TODO`, `TBD`, or "implement later" text remains in the task steps.
  - Optional `queries.go` and `scan.go` creation is tied to a concrete decision: create them only if helper extraction actually moves code there during Task 4.
- Type consistency check:
  - The helper names used in later steps (`normalizeRecallInput`, `createNormal`, `normalizeSearchInput`, `decodeOllamaEmbedding`) are introduced in the same task where they are used.

