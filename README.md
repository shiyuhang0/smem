# SMEM

[English](./README.en.md)

`smem` 是一个面向 agent 的长期记忆系统。

它面向个人部署，而不是托管式 SaaS。你可以完全掌控自己的数据与基础设施，并将服务运行在本地机器、私有云，或任意你选择的托管环境中。

`smem` 聚焦完整的记忆流水线：记忆提取、去重、融合、召回，以及与 agent 的集成。持久化并管理长期记忆。

![dashboard](./doc/dashboard.png)

## Benchmark

Data

- Sample: locomo `conv-26` (共10组 sample，取了第一组测试)
- Extracted memories: `260` in `419` turns

> Extracted based on GLM-5.1

Recall (top5)

| Dimension | Result |
|---|---|
| Sample | locomo `conv-26` |
| Questions | `199` |
| Retrieval p99 latency | `2.59s` |
| Accuracy | 64.8% |

See [recall details](./benchmark/locomo/out/recall/conv-26_top5_results.json) for more.

## 核心能力

- 安全：使用你自己的数据库、LLM、embedding 服务和 agent runtime
- 异步提取：不阻塞主 agent 路径
- 智能融合：基于 LLM 的记忆融合，支持创建、更新、删除和强化记忆
- 精确召回：
  - 粗排：vector search + full-text search + 可选 RRF
  - 精排：bge-rerank + 多维度打分
  - 可选发散机制：基于 softmax + temperature 的概率式召回，避免结果过度集中在少数记忆上
- Dashboard：用于浏览、搜索、过滤和归档记忆
- OpenClaw 插件：同时支持基于 tool 和基于 hook 的两种集成模式

## 快速开始

### 1. OpenClaw 插件启用

通过 npm 安装：

```bash
openclaw plugins install @shiyuhang0/smem-openclaw
```

OpenClaw 会自动写入类似如下的配置：

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

插件安装完成或配置变更后，重启 OpenClaw。

> 此时，插件已生效。但需启动 memory server 才能正常工作，见下一步。

### 2. 启动服务端

前置条件：

- Go `1.25+`
- TiDB Cloud 支持 `VECTOR` 和 `FULLTEXT`
- 一个兼容 OpenAI 的 chat model API
- 一个 embedding API（`openai` 或 `ollama`）

复制配置文件：

```bash
cp server/config.yaml.example server/config.yaml
```

编辑 `server/config.yaml`，填入数据库和模型配置。最小示例如下：

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

启动服务：

```bash
cd server
go run ./cmd/smem-server
```

## 架构

`smem` 采用 client + server 架构。
- 服务端：记忆管理，记忆提取，记忆召回
- client: 目前支持 openclaw plugin

```text
+-----------------------------+        HTTP API        +-----------------------------+        HTTP API        +----------------------+
| Agent Runtime               | <-------------------> | smem server                 | <-------------------> | dashboard            |
| (with smem client plugin)   |                       |                             |                       |                      |
|                             |                       | - 记忆提取                  |                       | - 查看记忆           |
| - 发起 recall/store         |                       | - 去重与融合                |                       | - 搜索 / 过滤        |
| - 调用 CRUD                 |                       | - 检索与 rerank             |                       | - 归档管理           |
+-----------------------------+                       | - 持久化与归档              |                       +----------------------+
                                                      +-------------+---------------+
                                                                    |
                                                                    v
                                              +-----------------------------------------+
                                              | TiDB Cloud + LLM + Embedding provider   |
                                              +-----------------------------------------+
```

## 工作方式

### Memory 分类

纵向+横向分类（type + kind）

type:
- `fact`：事实。
- `episodic`：发生过的事。
- `procedural`：流程、经验、约定。

kind:
- `skill`
- `task`
- `lesson`
- `workflow`
- `preference`
- `profile`
- `note`
- `decision`
- ...

### 异步 Ingest

所有记忆写入都会进入异步 ingest job 流水线。

- `POST /api/v1/memories` 会立即返回 `202 Accepted`
- 后台 worker 会在带重试机制的情况下安全处理任务
- 失败不会阻塞主 agent 路径
- 任务可以跨重启恢复

### Smart Ingest

`smem` 提供 `normal` 和 `smart` 两种 ingest 模式。`smart` 模式下会：

- 从输入中提取至多 5 条原子化候选记忆
- 基于候选记忆召回相关的已有记忆
- 基于 LLM 执行记忆融合，例如：
  - 无用记忆：忽略
  - 新记忆：创建
  - 冲突记忆：创建新建议并删除旧记忆
  - 记忆补充：补充旧记忆
  - 相同记忆：强化旧记忆，增加记忆次数 （content hash 去重）

### 精确召回

> 无大模型调用，秒级召回

- `vector search` 捕获语义相似性，`full-text search` 捕获词面匹配
- 可选 RRF：固定保留部分 top-k 结果，其余结果做 RRF 融合，兼顾单路保护与共识融合。`k` 值会按数据量动态调整
- `bge-rerank` + 多维度打分：以 rerank 分数为主，过滤低分结果，并以 `0.1` 的权重对其他维度做 boost，例如：
  - 时间（近期优先，7 天半衰期）：rerank 得分接近时，近期内容优先
  - 存储次数（多次记忆优先）：rerank 得分接近时，多次记忆优先。比如：我爱吃饭，我爱吃面（多次记忆），爱吃面优先召回
  - 类型
- 可选发散机制：通过 softmax 概率式选择，让召回结果更有多样性；`temperature` 用于控制发散程度

这让 `smem` 不仅能更精确地召回“相似”记忆，也更有机会返回在真实使用中“更有用”的记忆。

> 人会遗忘，但这是缺点，AI 为什么要学会遗忘？在 smem 中，记忆不会被遗忘，但近期记忆也更容易被召回。

### OpenClaw Tool Mode 与 Auto Mode

OpenClaw 插件可以替换 OpenClaw 的 `memory` slot，并接管记忆相关能力。

- 提供 `memory_search`、`memory_store` 等 tool。
- 支持两种集成方式：
  - `toolMode=true`：推荐默认值。模型以显式工具方式使用 `memory_search` 和 `memory_store`。system prompt 中会注入 guidance，引导模型在合适的时候主动调用这些工具。
  - `toolMode=false`：基于 hook 的自动模式。recall 在构建 prompt 前执行，store 在 `agent_end` 时执行。system prompt 会引导模型尽量不要主动调用工具，而是依赖自动 recall/store。自动模式下，召回内容会额外包裹在 `<memory>` 块中；在提取记忆时会移除该块，避免重复存储。
- 降级行为：recall 和 store 失败时都会静默降级，不影响主链路。

核心 inject point：

- `memory` slot 机制：通过 `kind: "memory"` 接入 OpenClaw 的排他 memory 插件体系，由 `plugins.slots.memory = "smem-openclaw"` 激活。
- `registerMemoryCapability({ promptBuilder })`：向 system prompt 注入静态 memory guidance，指导模型如何使用 memory。
- `registerTool`：注册 `memory_search` 和 `memory_store` tool。
- `api.on("before_prompt_build", ...)`：在每轮对话构建 prompt 前执行 recall，并注入召回内容。
- `api.on("agent_end", ...)`：在 agent 执行结束时触发 store。

### Dashboard

Dashboard 让你可以直接看到 agent 记住了什么。

- 查看记忆元数据和详情
- 归档记忆，而不是盲目删除历史

## 配置

### Server

服务端主配置位于 `server/config.yaml`。

关键字段：

- `server_addr`：HTTP 监听地址，默认 `:8080`
- `db_dsn`：TiDB 连接串
- `db_tls_server_name`：启用 TLS 时的 TiDB TLS server name
- `openai_base_url`：chat model base URL
- `openai_api_key`：chat model API key
- `openai_chat_model`：chat model 名称
- `rerank_provider`：当前仅支持 `siliconflow`
- `rerank_base_url`：rerank endpoint base URL
- `rerank_api_key`：rerank API key
- `rerank_model`：rerank model 名称
- `embedding_provider`：`openai` 或 `ollama`
- `embedding_base_url`：embedding endpoint base URL
- `embedding_api_key`：embedding API key
- `embedding_model`：embedding model 名称
- `embedding_dim`：embedding 维度

示例：

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

### OpenClaw 插件

插件支持以下配置项：

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

- `toolMode=true`：推荐默认值。模型以显式工具方式使用 `memory_search` 和 `memory_store`
- `toolMode=false`：基于 hook 的自动模式。每轮对话都会召回并存储记忆
- `serverURL`：SMEM server base URL，默认 `http://localhost:8080`
- `topK`：召回结果数量，默认 `5`
- `storeMode`：`normal` 或 `smart`，默认 `smart`
- `timeoutMs`：请求超时时间，单位毫秒，默认 `8000`

## Roadmap

- benchmark
- 多租户/权限
- 支持 vercel/netlify 部署
- CLI/MCP
- memory 来源记录（对话）
- 会话 digest 支持
- 时序优化：记忆创建时间不一定是事件发生时间，增加时间列可以更好地支持时间相关的记忆管理和召回策略。
- Ingest: 短期会话合并处理，否则拆开容易丢失信息
  

## 延伸阅读

- [`server/README.md`](./server/README.md)
- [`plugin/openclaw/README.md`](./plugin/openclaw/README.md)
- [`server/design.md`](./server/design.md)
- [`dashboard/design.md`](./dashboard/design.md)