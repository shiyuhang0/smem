# SMEM

`smem` 是一个面向 agent 的长期记忆系统。

它面向个人部署，而不是托管式 SaaS。因此你可以完全掌控自己的数据和基础设施。你可以把服务运行在本地机器、私有云，或者任意你选择的托管环境中。

- 使用你自己的数据库、LLM、embedding 服务和 agent runtime。
- 通过 memory server 持久化并管理长期记忆。
- 在 dashboard 中查看、搜索和归档记忆。

`smem` 聚焦完整的记忆流水线：记忆提取、去重、融合、召回和 agent 集成。

## 核心能力

- 记忆异步提取：不阻塞主 agent 路径。
- 智能记忆融合：基于 LLM 的记忆融合，支持创建、更新、删除和强化记忆。
- 多维召回
  - 基于 vector search + full-text search + RRF 粗排。
  - 精排：相关性为主，其他维度为辅的打分机制
  - 时间衰退机制
  - 低分过滤机制
  - 发散思维机制：softmax + temperature 的概率式召回选取，避免过度集中在少数记忆上。
- dashboard 用于浏览、搜索、过滤和归档记忆。
- openclaw 插件支持：提供基于 tool 和基于 hook 的两种集成模式。

## 快速开始

### 1. 启动服务端

前置条件：

- Go `1.25+`
- TiDB Cloud 或其他兼容 MySQL 的数据库，需要支持 `VECTOR` 和 `FULLTEXT` 。
- 一个兼容 OpenAI 的 chat model API
- 一个 embedding API（`openai` 或 `ollama`）

创建配置：

```bash
cp server/config.yaml.example server/config.yaml
```

编辑 `server/config.yaml`，填入你的数据库和模型配置。最简 example

```
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


然后运行：

```bash
cd server
go run ./cmd/smem-server
```

### 2. 安装 OpenClaw 插件

通过 npm 快速安装：

```bash
openclaw plugins install @shiyuhang0/smem-openclaw
```

OpenClaw 配置会自动配置

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

插件或配置变更后，重启 OpenClaw。

## 架构

`smem` 采用 client + server 架构。

- `server/`：Go 服务
- `plugin/openclaw/`：使用 TypeScript 实现的 OpenClaw memory 插件
- `dashboard/`：基于 React 的 memory dashboard

```text
用户 / Agent
    |
    v
OpenClaw + smem-openclaw plugin
    |
    v
smem server
    |
    v
TiDB Cloud + LLM + embedding provider
```

## 亮点

### 异步 Ingest

所有记忆写入都会经过异步 ingest job 流水线。

- `POST /api/v1/memories` 会立即返回 `202 Accepted`
- 后台 worker 会带重试地安全处理任务
- 失败不会阻塞主 agent 路径
- 任务可以跨重启恢复

### Smart Ingest

提供 `normal` 和 `smart` 两种 ingest 模式。smart 模式下：

- 从输入中提取至多 5 条原子化候选记忆
- 基于候选记忆召回相关的已有记忆
- 基于 LLM 进行记忆融合
  - 新记忆：创建
  - 冲突记忆：创建新建议，删除老记忆
  - 记忆补充：补充老记忆
  - 相同记忆：强化老记忆
  - ...

### 多维召回

- vector search 捕获语义相似性 + full-text search 捕获词面匹配 + RRF 合并两条召回通道
- 相似性为主，其他维度为辅的打分机制。即不会因为某个维度的高分就大幅提升最终排名，但会在相关性相近的记忆中起到区分作用。
  - 如多次记忆的内容会被优先召回：
- 引入时间衰退机制
- 引入低分过滤机制，过滤掉相关性过低的记忆
- 引入思维发散机制：Softmax 的概率式选择，以支持更有多样性的召回。调整 temperature 可以控制发散程度。

这让 `smem` 不只是返回“相似”的记忆，而是更有机会返回在实际使用中“更有用”的记忆。

### OpenClaw Tool Mode 与 Auto Mode

OpenClaw 插件支持两种集成方式。

- `toolMode=true`：推荐默认值。模型以显式工具方式使用 `memory_search` 和 `memory_store`
- `toolMode=false`：基于 hook 的自动模式。recall 在构建 prompt 前执行，store 在 `agent_end` 时执行

无论哪种模式，插件都会占用 OpenClaw 的 `memory` 插槽，并把当前活跃的 memory 路径替换为 `smem`。

### Dashboard

dashboard 让你可以直接看到 agent 记住了什么。

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

```
    "entries": {
      "smem-openclaw": {
        "enabled": true
         "config": {
           "serverURL": "http://localhost:8080",
           "toolMode": true,
           "topK": 5,
           "storeMode": "smart",
           "timeoutMs": 8000
         }
      }
    }

```

- toolMode=true：推荐默认值。模型以显式工具方式使用 memory_search 和 memory_store
- toolMode=false：基于 hook 的自动模式。每轮对话都会召回记忆，存储记忆。
- `serverURL`：SMEM server base URL。默认：`http://localhost:8080`
- `topK`：召回结果数量。默认：`5`
- `storeMode`：`normal` 或 `smart`。默认：`smart`
- `timeoutMs`：请求超时时间，单位毫秒。默认：`8000`


## 仓库结构

```text
server/             Go memory server
plugin/openclaw/    OpenClaw memory plugin
dashboard/          Memory dashboard
human_doc/          面向人的设计和研究文档
```

## 延伸阅读

- [`server/README.md`](./server/README.md)
- [`plugin/openclaw/README.md`](./plugin/openclaw/README.md)
- [`server/design.md`](./server/design.md)
- [`dashboard/design.md`](./dashboard/design.md)
