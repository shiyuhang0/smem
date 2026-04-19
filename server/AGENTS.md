# AGENTS Guide

This file is for coding agents working in `smem/apps/server`.

## Tech Stack

- Go 1.25
- Gin (HTTP framework)
- GORM (ORM)
- TiDB Cloud (production storage, with native VECTOR and FULLTEXT support)
- SQLite in-memory (test-only storage)
- OpenAI-compatible HTTP APIs for LLM and embeddings
- Module path: `smem/apps/server`

## Working Directory

All commands in this file must be run from `server/`.

```bash
cd server
```

## Build Commands

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
go test ./internal/domain/ingest -run TestJobWorkerRunOnceSmartModeUpdatesExistingMemory -v
go test ./internal/domain/recall -run TestRecallMergesVectorAndFullTextViaRRFAndReranks -v
go test ./internal/store -run TestIngestJobRepository -v
go test ./internal/handler -run TestMemoryHandlerCreateAndGet -v
```

Disable test caching when debugging:

```bash
go test ./internal/handler -count=1 -v
```

TiDB Cloud integration test (gated by env var):

```bash
SMEM_INTEGRATION_TIDB=1 DB_DSN='...' DB_TLS_SERVER_NAME='...' go test ./internal/store -run TestTiDBCloudConnection -v
```

## Formatting / Linting

No dedicated linter. Use Go formatting and tests as baseline:

```bash
gofmt -w ./cmd ./internal
go test ./...
```

If you touch imports, run `gofmt`; do not hand-format import blocks.

## Dependency Management

```bash
go mod tidy
```

Do not edit `go.mod` or `go.sum` manually unless absolutely necessary.

## Architecture

```
cmd/smem-server/              程序入口，加载配置、启动服务、优雅关闭
internal/
├── app/                      依赖注入（New）、路由注册（NewRouter）、优雅关闭
├── config/                   环境变量 + YAML 配置加载（Load），日志工具（NewLogger）
├── handler/                  Gin HTTP Handler、请求/响应 DTO、错误映射
│   ├── memory_handler.go     POST/GET/PUT/DELETE /api/v1/memories, POST .../recall
│   ├── request.go            createMemoryRequest, updateMemoryRequest, recallRequest
│   ├── response.go           memoryResponse, listMemoriesResponse, ingestJobResponse
│   └── error.go              ErrorResponse, writeError（domain 错误 → HTTP 状态码）
├── store/                    GORM 模型、Repository 实现、数据库连接、迁移
│   ├── model.go              MemoryModel, IngestJobModel, 自定义类型（Float32Slice, StringSlice, JSONMap）
│   ├── repository.go         memory.Repository 的 GORM 实现（CRUD, VectorSearch, FullTextSearch, UpsertByContentHash）
│   ├── ingest_job_repository.go  ingestjob.Repository 的 GORM 实现（Submit, ClaimNext, MarkRetry/Failed/Succeeded）
│   ├── connector.go          PrepareDSN（TiDB TLS 注册、ParseTime 注入）
│   ├── migration.go          ApplyMigrations（按序执行 migrations/*.sql）
│   └── transaction_manager.go    Run（事务内同时操作 memory 和 ingestjob repo）
├── ai/
│   ├── llm/
│   │   ├── provider.go       Provider 接口（GenerateText）
│   │   ├── openai.go         OpenAIProvider（/chat/completions，retry 包装）
│   │   └── prompt.go         信息提取 prompt（NewExtractionPrompt）+ 记忆融合 prompt（NewFusionDecisionPrompt）
│   ├── embedding/
│   │   ├── provider.go       Provider 接口（Embed）
│   │   ├── openai.go         OpenAIProvider（/embeddings）
│   │   └── ollama.go         OllamaProvider（/api/embed）
│   └── retry/
│       └── policy.go         共享重试策略（3 次、指数退避 + 抖动、429/5xx 重试）
└── domain/
    ├── memory/
    │   ├── entity.go         Memory 实体、CreateInput/UpdateInput/ListInput/RecallInput/RecallCandidate/RecallResult
    │   ├── enum.go           Type（fact/episodic/procedural）、Scope（user/agent/external）、State（creating/active/archived）、Mode（normal/smart）
    │   ├── repository.go     Repository 接口（Create, UpsertByContentHash, Update, Delete, GetByID, List, Search, VectorSearch, FullTextSearch）
    │   ├── service.go        Service（CRUD 业务逻辑）
    │   └── validate.go       CreateInput / RecallInput 校验
    ├── ingestjob/
    │   ├── entity.go         Job 实体（异步 ingest 任务）
    │   ├── enum.go           State（pending/running/succeeded/failed）、Mode（normal/smart）
    │   └── repository.go     Repository 接口（Submit, ClaimNext, MarkRetry, MarkFailed, MarkSucceeded）
    ├── ingest/
    │   ├── service.go        Service.Create（校验 + 提交 pending job）
    │   ├── job_worker.go     JobWorker（轮询执行 normal/smart job，事务写入，失败重试）
    │   ├── job_types.go      memoryWriteSet, writeOp（中间数据结构）
    │   └── parser.go         parseExtractionPayload, parseFusionPayload（LLM JSON 解析 + 校验）
    └── recall/
        ├── service.go        Service.Recall（embed → 双路搜索 → RRF → 精排 → topK）
        └── scoring/
            ├── rrf.go        RRF 融合（k=60）
            ├── score.go      复合打分（relevance-gated: vector 0.6 + fulltext 0.4, boost: recency 0.7 + store_count 0.3）
            └── softmax.go    Softmax + 概率 topK 选择（已实现，默认关闭）
```

### 分层规则

- **handler** 负责 HTTP 关注点（绑定请求、映射错误、返回响应），不包含业务逻辑。
- **domain** 包含纯业务逻辑，不依赖 Gin 或 GORM。
- **store** 实现 domain 层定义的 Repository 接口。
- **app** 负责依赖注入，将所有层组装在一起。
- 不要将 HTTP 关注点（状态码、请求绑定）放进 domain 包。

### 依赖注入

`internal/app/app.go` 中的 `New(cfg)` 按顺序创建：

1. `store.PrepareDSN` → `gorm.Open` → `store.ApplyMigrations`
2. `store.NewRepository` + `store.NewIngestJobRepository` + `store.NewTransactionManager`
3. `memory.NewService` + `retry.DefaultPolicy`
4. `llm.NewOpenAIProvider` + `embedding.NewOpenAIProvider` 或 `NewOllamaProvider`
5. `recall.NewService` + `ingest.NewService` + `ingest.NewJobWorker`
6. `jobWorker.Start(ctx)` — 后台 goroutine 每秒轮询
7. `handler.NewMemoryHandler` + `NewRouter`

关闭顺序：cancel worker context → shutdown HTTP server (5s timeout) → close DB。

## API

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/healthz` | inline | `{"status":"ok"}` |
| POST | `/api/v1/memories` | create | 提交 ingest job，返回 202 + IngestJob |
| GET | `/api/v1/memories` | list | 分页列表，支持 search/state/type 过滤 |
| GET | `/api/v1/memories/:id` | get | 查询单条 memory |
| PUT | `/api/v1/memories/:id` | update | 更新 memory |
| DELETE | `/api/v1/memories/:id` | delete | 删除 memory |
| POST | `/api/v1/memories/recall` | recallMemories | 召回相关记忆 |

OpenAPI 定义见 `api/openapi.yaml`。

## Key Business Flows

### Normal Ingest

1. `POST /api/v1/memories` with `mode: "normal"` → handler 调用 `ingestService.Create` → 提交 pending job → 返回 202
2. `JobWorker.RunOnce` 每秒轮询，ClaimNext 获取 pending job
3. `executeNormalJob`：构建 Memory（state=active）→ 计算 content_hash → 调用 embedder.Embed → `UpsertByContentHash`（hash 冲突则 store_count++）
4. 事务内标记 job succeeded

### Smart Ingest

1. 同样提交 pending job，mode=smart
2. `executeSmartJob` 四阶段：
   - **Phase 1 - 提取**：LLM + extraction prompt → 最多 5 条原子化候选记忆（content + type + kinds）
   - **Phase 2 - 召回**：对每条候选内容调用 recall.Recall（top 3），去重合并
   - **Phase 3 - 融合**：LLM + fusion prompt → 对每条候选/已有记忆输出 action（candidate: ignore/create, memory: update/delete/ignore）
   - **Phase 4 - 写入**：`buildWriteSet` 处理 action → 事务内执行 create/update/delete
3. 失败重试最多 5 次，线性退避

### Recall

`POST /api/v1/memories/recall` with `{content, top_k, temperature}` → `recallService.Recall`:

1. **normalize**：默认 topK=5, temperature=1
2. **embed query** → 向量搜索 topK×2 条 + 全文搜索 topK×2 条（仅 active 状态）
3. **RRF 融合**（k=60）→ 合并两路结果 → 取 top topK×2
4. **Relevance-Gated Rerank**：
   - `relevance = 0.6 × vector_similarity + 0.4 × fulltext_score`
   - 若 relevance < 0.2：不加 boost，直接返回 relevance（防止弱相关记忆被抬分）
   - 若 relevance ≥ 0.2：`boost = 0.7 × recency + 0.3 × store_count_score`，final = `relevance + 0.1 × boost`
5. **Top-K 选择**：按得分取前 K 条（Softmax 概率选择已实现，当前关闭）

### Job Lifecycle

- **ClaimNext**：乐观锁并发 Claim（检查 state + execute_count + next_run_at），冲突时重试最多 5 次
- **MarkRetry**：execute_count < 5 时，设 state=pending, next_run_at=now+execute_count 秒
- **MarkFailed**：execute_count ≥ 5 时，永久标记 failed
- **MarkSucceeded**：记录 result_memory_ids 和 result_summary
- 所有 terminal 操作检查 `state=running AND execute_count=? AND worker_id=?`

## Config

通过环境变量或 YAML 文件配置（`config.yaml` 或 `SMEM_CONFIG_FILE` 指定路径）。YAML 值覆盖环境变量。

| 字段 | 环境变量 | 默认值 | 必填 |
|------|---------|--------|------|
| `server_addr` | `SERVER_ADDR` | `:8080` | 否 |
| `db_dsn` | `DB_DSN` | - | 是 |
| `db_tls_server_name` | `DB_TLS_SERVER_NAME` | - | 否（设置后启用 TiDB TLS） |
| `openai_base_url` | `OPENAI_BASE_URL` | `https://api.openai.com/v1` | 否 |
| `openai_api_key` | `OPENAI_API_KEY` | - | 是 |
| `openai_chat_model` | `OPENAI_CHAT_MODEL` | `gpt-4.1-mini` | 否 |
| `embedding_provider` | `EMBEDDING_PROVIDER` | `ollama` | 是（openai 或 ollama） |
| `embedding_base_url` | `EMBEDDING_BASE_URL` | Provider 默认值 | 否 |
| `embedding_api_key` | `EMBEDDING_API_KEY` | OpenAI 时回退到 openai_api_key | 否 |
| `embedding_model` | `EMBEDDING_MODEL` | `text-embedding-3-small` / `bge-m3` | 否 |
| `embedding_dim` | `EMBEDDING_DIM` | `1536` / `1024` | 是（>0） |

示例见 `config.yaml.example`。

## Database

- 生产环境使用 **TiDB Cloud**，依赖原生 `VECTOR(dim)` 列、`vec_cosine_distance()` 函数、`FULLTEXT ... WITH PARSER MULTILINGUAL` 索引、`fts_match_word()` 函数。
- 测试使用 **SQLite in-memory**（`gorm.Open(sqlite.Open("file::memory:?cache=shared"))`）。
- 迁移文件在 `migrations/`，按文件名排序执行，`already exists` 和 `duplicate key name` 错误被忽略。
- `content_hash` 列有唯一索引，用于 `UpsertByContentHash` 去重。

## Code Style

### General

- 小而专注的文件。
- 最小化变更，不做不必要的架构重构。
- 逻辑靠近使用处，除非明确需要复用。
- 匹配已有的命名和包边界。

### Flow Style（参考 recall/service.go 的 Recall 方法）

- 顶层方法保持线性可读：normalize → 主流程各阶段 → 返回。
- 在重要决策点、权衡处、未来扩展点添加注释。
- 关键阶段输出简洁日志（使用 `config.NewLogger`），方便运行时诊断。
- 提取有意义的 helper 方法，保持顶层方法清晰。

### Formatting

- 始终使用 `gofmt`。
- Tab 缩进（Go 工具期望）。
- 避免密集的单行代码。

### Imports

- 让 `gofmt` 管理 import 分组。
- 标准库 → 第三方 → 本模块。
- 仅在需要解决冲突或澄清意图时使用别名。

### Naming

- 导出：`PascalCase`，未导出：`camelCase`。
- 包名短小写，避免重复（`memory.Service` 而非 `memory.MemoryService`）。
- 方法名描述性：`Create`、`Recall`、`PrepareDSN`、`GenerateText`、`Embed`。

### Types

- 领域概念使用类型化枚举：`memory.Type`、`memory.Scope`、`memory.State`、`memory.Mode`。
- 优先使用具体 struct，避免 `map[string]any`（除非形状真正动态）。
- 请求/更新 struct 中仅当 omitted vs zero value 有区别时使用指针。
- JSON DTO 与领域实体分开。

### Error Handling

- 返回 error，不在生产路径 panic。
- 使用 `fmt.Errorf("...: %w", err)` 包装上下文。
- 调用方需要分支时使用哨兵错误：`memory.ErrNotFound`、`ingestjob.ErrNotFound`、`ingestjob.ErrConflict`。
- 在 handler 层统一映射 domain/storage 错误到 HTTP 状态码。

### Context, Time, IDs

- `context.Context` 穿透 service / repository / LLM / embedding 边界。
- Handler 中使用 `c.Request.Context()`，不向下传递 `*gin.Context`。
- UTC 时间戳。
- 测试需要确定性时注入 clock/ID 函数。

### Testing

- 行为变更前先写测试。
- 包内测试验证内部逻辑。
- 校验和分支密集逻辑使用 table-driven tests。
- 使用 `require` 做硬断言和 setup 检查。
- Repository 测试使用 SQLite 内存数据库。
- 集成测试必须通过环境变量门控（`SMEM_INTEGRATION_TIDB=1`）。
- 为测试创建轻量 fake 实现（`memoryRepo`、`fakeEmbedder`、`fakeLLMProvider` 等）。
- 注入 `now`、`id`、`randFloat` 函数以实现确定性测试。

## Conventions

- Smart ingest 在 LLM 提取失败时必须干净降级为 normal ingest。
- LLM 和 embedding 调用使用共享重试策略：3 次尝试，仅重试 retryable 失败（429、5xx、网络错误）。
- `creating` 状态的记忆不可搜索、不可召回。
- TiDB TLS 通过 `DB_DSN` 和 `DB_TLS_SERVER_NAME` 配置。
- `content_hash`（SHA-256）用于精确去重，冲突时 `store_count++`。
- LLM 返回的 kinds 中不在白名单的值被静默过滤（`filterKinds`）。
- ID 格式：`mem-<16hex>`、`ing-<16hex>`、`worker-<16hex>`。

## Before Finishing

From `server/`:

```bash
gofmt -w ./cmd ./internal
go test ./...
go build ./cmd/smem-server
```

If you changed DB connectivity or TiDB TLS behavior and credentials are available:

```bash
SMEM_INTEGRATION_TIDB=1 DB_DSN='...' DB_TLS_SERVER_NAME='...' go test ./internal/store -run TestTiDBCloudConnection -v
```

Do not claim completion without fresh command output.
