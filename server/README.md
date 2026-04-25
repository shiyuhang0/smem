# smem-server

Agent 记忆系统服务端，提供 memory 的存储、管理、智能写入与召回能力。

## Overview

- Tech stack: Go, Gin, GORM, TiDB Cloud
- AI integration: OpenAI-compatible LLM, embedding, rerank
- Storage: TiDB `VECTOR` + `FULLTEXT`
- Write path: async ingest job + background worker
- Recall path: vector search + full-text search + rerank

## Architecture

```text
cmd/
└── smem-server/      程序入口

internal/
├── app/              依赖注入、路由、优雅关闭
├── handler/          HTTP handler 和 DTO
├── store/            GORM repository、连接、迁移
├── ai/               LLM / embedding / retry
└── domain/
    ├── memory/       Memory 实体、枚举、CRUD
    ├── ingestjob/    异步 job 生命周期
    ├── ingest/       normal/smart 写入编排、worker
    └── recall/       搜索、合并候选、rerank、排序
```

分层原则：`handler` 只处理 HTTP，`domain` 不依赖具体存储，`store` 实现 repository。

## Quick Start

### Requirements

- Go 1.25+
- TiDB Cloud with `VECTOR` and `FULLTEXT`
- LLM API
- Embedding API
- Rerank API

### Config

```bash
cp config.yaml.example config.yaml
```

核心配置：

- `DB_DSN`
- `DB_TLS_SERVER_NAME`
- `OPENAI_API_KEY`
- `RERANK_API_KEY`
- `EMBEDDING_PROVIDER`
- `EMBEDDING_MODEL`
- `EMBEDDING_DIM`

完整示例见 `config.yaml.example`。

### Run

```bash
cd server
go build ./cmd/smem-server
./smem-server
```

或：

```bash
go run ./cmd/smem-server
```

- Health check: `GET /healthz`
- Default addr: `:8080`

## API

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/memories` | 创建 memory，提交 ingest job，返回 `202` |
| `GET` | `/api/v1/memories` | 列表/搜索，支持分页和 `kind/state/type` 过滤 |
| `GET` | `/api/v1/memories/:id` | 查询单条 memory |
| `PUT` | `/api/v1/memories/:id` | 更新 memory |
| `DELETE` | `/api/v1/memories/:id` | 删除 memory |
| `POST` | `/api/v1/memories/recall` | 召回相关 memory |

OpenAPI: `api/openapi.yaml`

## Ingest

所有 `POST /api/v1/memories` 都先写入 ingest job，再由后台 worker 处理。

### Normal Mode

- 直接写入 memory
- 生成 embedding 和 `content_hash`
- `content_hash` 冲突时转为累加 `store_count`

### Smart Mode

流程：

1. LLM 提取最多 5 条原子化候选记忆
2. 对每条候选召回最多 3 条已有记忆
3. LLM 输出融合动作
4. 事务内执行 `create / update / delete / ignore`

失败最多重试 5 次。

## Recall

当前召回流程：

1. query embedding
2. 向量搜索 `4K`
3. 全文搜索 `4K`
4. 合并两路候选并去重
5. rerank 过滤低分候选
6. 叠加轻量 boost 后取 Top-K

说明：

- 默认不是 RRF 融合；RRF 逻辑已实现，但当前关闭
- 当前 boost 主要使用 `recency` 和 `store_count`
- Softmax 选择已实现，但默认关闭

## Providers

### LLM

- OpenAI-compatible `/chat/completions`

### Rerank

- Current provider: SiliconFlow
- Default model: `BAAI/bge-reranker-v2-m3`

### Embedding

| Provider | Default Model | Dim |
|---|---|---|
| OpenAI | `text-embedding-3-small` | `1536` |
| Ollama | `bge-m3` | `1024` |

## Development

```bash
cd server
gofmt -w ./cmd ./internal
go test ./...
go build ./cmd/smem-server
```
