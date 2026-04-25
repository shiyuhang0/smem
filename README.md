# SMEM

[中文文档](./README.zh-CN.md)

`smem` is a long-term memory system for agents.

It is designed for self-hosting rather than as a managed SaaS product. You keep full control over your data and infrastructure, and can run it on your local machine, private cloud, or any hosting environment you choose.

`smem` focuses on the full memory pipeline: memory extraction, deduplication, consolidation, retrieval, and agent integration.

## What It's For

- Use your own database, LLM, embedding service, and agent runtime
- Persist and manage long-term memory through the memory server
- View, search, filter, and archive memories in the dashboard

## Core Capabilities

- Asynchronous extraction: does not block the main agent path
- Smart consolidation: LLM-based memory merging with support for creating, updating, deleting, and reinforcing memories
- Precise retrieval:
  - Initial retrieval: vector search + full-text search + optional RRF
  - Reranking: bge-rerank + multi-factor scoring
  - Optional diversification: probabilistic retrieval based on softmax + temperature to avoid over-concentrating on a small set of memories
- Dashboard: browse, search, filter, and archive memories
- OpenClaw plugin: supports both tool-based and hook-based integration modes

## Quick Start

### 1. Start the Server

Prerequisites:

- Go `1.25+`
- TiDB Cloud or another MySQL-compatible database with `VECTOR` and `FULLTEXT` support
- An OpenAI-compatible chat model API
- An embedding API (`openai` or `ollama`)

Copy the config file:

```bash
cp server/config.yaml.example server/config.yaml
```

Edit `server/config.yaml` and fill in your database and model settings. A minimal example:

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

Install it with npm:

```bash
openclaw plugins install @shiyuhang0/smem-openclaw
```

OpenClaw will automatically write a config similar to:

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

Restart OpenClaw after installation or any config change.

## Architecture

`smem` uses a client + server architecture.

The client is injected into the agent as a plugin, with responsibilities cleanly separated:

- The server focuses on memory processing
- The client is responsible for calling memory CRUD APIs
- Client support currently includes the OpenClaw plugin

- `server/`: Go service
- `plugin/openclaw/`: OpenClaw memory plugin implemented in TypeScript
- `dashboard/`: React-based memory dashboard

```text
+-----------------------------+        HTTP API        +-----------------------------+        HTTP API        +----------------------+
| Agent Runtime               | <-------------------> | smem server                 | <-------------------> | dashboard            |
| (with smem client plugin)   |                       |                             |                       |                      |
|                             |                       | - memory extraction         |                       | - inspect memories   |
| - trigger recall/store      |                       | - dedup/consolidation       |                       | - search / filter    |
| - call CRUD APIs            |                       | - retrieval / rerank        |                       | - archive management |
+-----------------------------+                       | - persistence / archive     |                       +----------------------+
                                                      +-------------+---------------+
                                                                    |
                                                                    v
                                              +-----------------------------------------+
                                              | TiDB Cloud + LLM + Embedding provider   |
                                              +-----------------------------------------+
```

## How It Works

### Async Ingest

All memory writes go through an asynchronous ingest job pipeline.

- `POST /api/v1/memories` returns `202 Accepted` immediately
- Background workers process jobs safely with retries
- Failures do not block the main agent path
- Jobs can recover across restarts

### Smart Ingest

`smem` provides two ingest modes: `normal` and `smart`. In `smart` mode it will:

- Extract up to 5 atomic candidate memories from the input
- Retrieve related existing memories based on those candidates
- Use an LLM to consolidate memories, for example:
  - New memory: create
  - Conflicting memory: create a new suggestion and delete the old memory
  - Memory enrichment: update and extend the old memory
  - Identical memory: reinforce the old memory

### Precise Retrieval

- `vector search` captures semantic similarity, while `full-text search` captures lexical matches
- Optional RRF: keep a fixed subset of top-k results, then apply RRF to the remaining candidates to balance single-source protection and consensus fusion. The `k` value is adjusted dynamically based on data size
- `bge-rerank` + multi-factor scoring: rerank score is primary, low-score results are filtered out, and other factors receive a `0.1` weight boost, such as:
  - Time (recent memories preferred, with a 7-day half-life)
  - Storage count (memories seen more often are preferred)
  - Type
- Optional diversification: use softmax-based probabilistic selection to increase result diversity, with `temperature` controlling the degree of diversification

This allows `smem` not only to retrieve memories that are more precisely “similar”, but also to return memories that are more practically “useful” in real usage.

### OpenClaw Tool Mode and Auto Mode

The OpenClaw plugin can replace OpenClaw's `memory` slot and take over memory-related capabilities.

- It provides tools such as `memory_search` and `memory_store`.
- It supports two integration modes:
  - `toolMode=true`: recommended default. The model uses `memory_search` and `memory_store` explicitly as tools. Guidance is injected into the system prompt to help the model call these tools when appropriate.
  - `toolMode=false`: hook-based automatic mode. Recall runs before prompt construction, and storage runs at `agent_end`. The system prompt encourages the model not to call tools proactively, but to rely on automatic recall/store instead. In automatic mode, recalled content is wrapped in a `<memory>` block, and that block is removed during memory extraction to avoid duplicate storage.
- Degradation behavior: both recall and store fail silently and do not affect the main execution path.

Core inject points:

- `memory` slot mechanism: integrates with OpenClaw's exclusive memory plugin system through `kind: "memory"`, and is activated by `plugins.slots.memory = "smem-openclaw"`.
- `registerMemoryCapability({ promptBuilder })`: injects static memory guidance into the system prompt to guide the model in using memory.
- `registerTool`: registers the `memory_search` and `memory_store` tools.
- `api.on("before_prompt_build", ...)`: performs recall before each prompt is built and injects the recalled content.
- `api.on("agent_end", ...)`: triggers storage when the agent run ends.

### Dashboard

The dashboard lets you directly inspect what the agent has remembered.

- View memory metadata and details
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
- `rerank_provider`: currently only `siliconflow` is supported
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

The plugin supports the following configuration:

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

- `toolMode=true`: recommended default. The model uses `memory_search` and `memory_store` explicitly as tools
- `toolMode=false`: hook-based automatic mode. Each conversation round performs both retrieval and storage
- `serverURL`: SMEM server base URL, default `http://localhost:8080`
- `topK`: number of retrieval results, default `5`
- `storeMode`: `normal` or `smart`, default `smart`
- `timeoutMs`: request timeout in milliseconds, default `8000`

## Further Reading

- [`server/README.md`](./server/README.md)
- [`plugin/openclaw/README.md`](./plugin/openclaw/README.md)
- [`server/design.md`](./server/design.md)
- [`dashboard/design.md`](./dashboard/design.md)
