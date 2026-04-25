# SMEM

[中文](./README.md)

`smem` is a long-term memory system for agents.

It is designed for self-hosting rather than managed SaaS. You keep full control over your data and infrastructure, and can run it on your local machine, private cloud, or any hosting environment you choose.

`smem` focuses on the full memory pipeline: extraction, deduplication, fusion, recall, and agent integration. It persists and manages long-term memory.

![dashboard](./doc/dashboard.png)

## Core Capabilities

- Secure by design: use your own database, LLM, embedding service, and agent runtime
- Async extraction: does not block the main agent path
- Smart fusion: LLM-based memory fusion with create, update, delete, and reinforcement flows
- Precise recall:
  - Coarse stage: vector search + full-text search + optional RRF
  - Fine stage: `bge-rerank` + multi-signal scoring
  - Optional diversification: probabilistic recall with softmax + temperature to avoid over-concentrating on a few memories
- Dashboard: browse, search, filter, and archive memories
- OpenClaw plugin: supports both tool-based and hook-based integration modes

## Quick Start

### 1. Start the Server

Prerequisites:

- Go `1.25+`
- TiDB Cloud with `VECTOR` and `FULLTEXT` support
- An OpenAI-compatible chat model API
- An embedding API (`openai` or `ollama`)

Copy the config file:

```bash
cp server/config.yaml.example server/config.yaml
```

Edit `server/config.yaml` and fill in your database and model settings. Minimal example:

```yaml
db_dsn: "user:password@tcp(host:4000)/smem"
db_tls_server_name: "<db host>"

openai_base_url: "https://api.openai.com/v1"
openai_api_key: "your-api-key"
openai_chat_model: "gpt-5.4"

rerank_provider: "siliconflow"
rerank_base_url: "https://api.siliconflow.cn/v1"
rerank_api_key: "your-rerank-api-key"
rerank_model: "BAAI/bge-reranker-v2-m3"

embedding_provider: "openai"
embedding_base_url: "https://api.openai.com/v1"
embedding_api_key: "your-api-key"
embedding_model: "text-embedding-3-small"
```

Start the server:

```bash
cd server
go run ./cmd/smem-server
```

### 2. Install the OpenClaw Plugin

Install via npm:

```bash
openclaw plugins install @shiyuhang0/smem-openclaw
```

OpenClaw will automatically write config similar to:

```json
{
  "plugins": {
    "enabled": true,
    "slots": {
      "memory": "smem-openclaw"
    },
    "entries": {
      "smem-openclaw": {
        "enabled": true
      }
    }
  }
}
```

Restart OpenClaw after installation or config changes.

## Architecture

`smem` uses a client + server architecture.

- Server: memory management, memory extraction, memory recall
- Client: currently supports the OpenClaw plugin

```text
+-----------------------------+        HTTP API        +-----------------------------+        HTTP API        +----------------------+
| Agent Runtime               | <-------------------> | smem server                 | <-------------------> | dashboard            |
| (with smem client plugin)   |                       |                             |                       |                      |
|                             |                       | - memory extraction         |                       | - inspect memories   |
| - trigger recall/store      |                       | - dedup and fusion          |                       | - search / filter    |
| - call CRUD                 |                       | - retrieval and rerank      |                       | - archive management |
+-----------------------------+                       | - persistence and archive   |                       +----------------------+
                                                      +-------------+---------------+
                                                                    |
                                                                    v
                                              +-----------------------------------------+
                                              | TiDB Cloud + LLM + Embedding provider   |
                                              +-----------------------------------------+
```

## How It Works

### Memory Classification

Vertical + horizontal classification (`type` + `kind`)

`type`:

- `fact`: facts
- `episodic`: events or experiences
- `procedural`: workflows, practices, and conventions

`kind`:

- `skill`
- `task`
- `lesson`
- `workflow`
- `preference`
- `profile`
- `note`
- `decision`
- ...

### Async Ingest

All memory writes go through an async ingest job pipeline.

- `POST /api/v1/memories` returns `202 Accepted` immediately
- A background worker processes jobs safely with retries
- Failures do not block the main agent path
- Jobs can recover across restarts

### Smart Ingest

`smem` supports two ingest modes: `normal` and `smart`. In `smart` mode it will:

- Extract up to 5 atomic candidate memories from the input
- Recall related existing memories for those candidates
- Use an LLM to fuse memories, for example:
  - useless memory: ignore
  - new memory: create
  - conflicting memory: create a new suggestion and delete the old one
  - complementary memory: enrich the old memory
  - same memory: reinforce the old memory and increase its count (`content_hash` dedupe)

### Precise Recall

> No LLM call in the recall path, with second-level latency

- `vector search` captures semantic similarity, while `full-text search` captures literal matching
- Optional RRF: keep part of the top-k results fixed, and fuse the rest with RRF to balance single-source protection and cross-source consensus. The `k` value adjusts dynamically with data size
- `bge-rerank` + multi-signal scoring: rerank score is primary, low-score results are filtered, and other dimensions are boosted with a `0.1` weight, for example:
  - recency (recent memories first, 7-day half-life): when rerank scores are close, recent content wins
  - store count (frequently reinforced memories first): when rerank scores are close, reinforced memories rank higher
  - type
- Optional diversification: probabilistic selection with softmax so recall results are more diverse; `temperature` controls the exploration level

This allows `smem` to not only recall "similar" memories more accurately, but also surface memories that are more useful in real usage.

### OpenClaw Tool Mode and Auto Mode

The OpenClaw plugin can replace OpenClaw's `memory` slot and take over memory-related capabilities.

- Provides tools like `memory_search` and `memory_store`
- Supports two integration modes:
  - `toolMode=true`: recommended default. The model explicitly uses `memory_search` and `memory_store`. Guidance is injected into the system prompt so the model can proactively call these tools when appropriate.
  - `toolMode=false`: hook-based automatic mode. Recall runs before prompt building, and store runs on `agent_end`. The system prompt guides the model to avoid calling tools directly and rely on automatic recall/store instead. In auto mode, recalled content is wrapped in a `<memory>` block; that block is removed during extraction to avoid duplicate storage.
- Fallback behavior: recall and store failures degrade silently and do not break the main path

Core inject points:

- `memory` slot: integrates through OpenClaw's exclusive memory plugin mechanism via `plugins.slots.memory = "smem-openclaw"`
- `registerMemoryCapability({ promptBuilder })`: injects static memory guidance into the system prompt
- `registerTool`: registers `memory_search` and `memory_store`
- `api.on("before_prompt_build", ...)`: runs recall before each prompt build and injects recalled content
- `api.on("agent_end", ...)`: triggers store after the agent finishes

### Dashboard

The dashboard lets you see what the agent has remembered.

- Inspect memory metadata and details
- Archive memories instead of blindly deleting history

## Configuration

### Server

The main server config lives in `server/config.yaml`.

Key fields:

- `server_addr`: HTTP listen address, default `:8080`
- `db_dsn`: TiDB connection string
- `db_tls_server_name`: TiDB TLS server name when TLS is enabled
- `openai_base_url`: chat model base URL
- `openai_api_key`: chat model API key
- `openai_chat_model`: chat model name
- `rerank_provider`: currently only `siliconflow`
- `rerank_base_url`: rerank endpoint base URL
- `rerank_api_key`: rerank API key
- `rerank_model`: rerank model name
- `embedding_provider`: `openai` or `ollama`
- `embedding_base_url`: embedding endpoint base URL
- `embedding_api_key`: embedding API key
- `embedding_model`: embedding model name
- `embedding_dim`: embedding dimension

Example:

```yaml
server_addr: ":8080"

db_dsn: "user:password@tcp(gateway01.ap-southeast-1.prod.aws.tidbcloud.com:4000)/smem"
db_tls_server_name: "gateway01.ap-southeast-1.prod.aws.tidbcloud.com"

openai_base_url: "https://api.openai.com/v1"
openai_api_key: "your-api-key"
openai_chat_model: "gpt-5.4"

rerank_provider: "siliconflow"
rerank_base_url: "https://api.siliconflow.cn/v1"
rerank_api_key: "your-rerank-api-key"
rerank_model: "BAAI/bge-reranker-v2-m3"

embedding_provider: "openai"
embedding_base_url: "https://api.openai.com/v1"
embedding_api_key: "your-api-key"
embedding_model: "text-embedding-3-small"
embedding_dim: 1536
```

### OpenClaw Plugin

The plugin supports the following config:

```json
{
  "entries": {
    "smem-openclaw": {
      "enabled": true,
      "config": {
        "serverURL": "http://localhost:8080",
        "toolMode": true,
        "topK": 5,
        "storeMode": "smart",
        "timeoutMs": 8000
      }
    }
  }
}
```

- `toolMode=true`: recommended default. The model explicitly uses `memory_search` and `memory_store`
- `toolMode=false`: hook-based automatic mode. Every conversation turn performs recall and store automatically
- `serverURL`: SMEM server base URL, default `http://localhost:8080`
- `topK`: number of recall results, default `5`
- `storeMode`: `normal` or `smart`, default `smart`
- `timeoutMs`: request timeout in milliseconds, default `8000`

## Further Reading

- [`README.zh-CN.md`](./README.zh-CN.md)
- [`README.en.md`](./README.en.md)
- [`server/README.md`](./server/README.md)
- [`plugin/openclaw/README.md`](./plugin/openclaw/README.md)
- [`server/design.md`](./server/design.md)
- [`dashboard/design.md`](./dashboard/design.md)
