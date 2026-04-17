# Overview

`smem` 的目标是实现一个用于 Agent 的记忆系统。

# Goals

1. 可用 demo：可用，但质量要高于 demo。实现 openclaw plugin + TiDB Cloud 存储。
2. 个人控制：仅用于个人搭建，不做服务。用户对数据完全控制，用户提供必要执行环境，如数据库、LLM、Agent 运行环境等。
3. 创新：本项目用于个人学习，需要对比市面常见记忆系统，需要有一定创新点/难点。

# Non-goals

- 服务化：包括权限、服务端全托管等。
- 其他 agent 插件、其他存储。

# Constraints

- 服务端本地运行: 用户需自行提供 LLM API key 和数据库连接信息。服务端使用 Go 实现。
- 客户端: openclaw 设计 memory plugin，使用 TypeScript 实现。

# Glossary

- memory：系统中的记忆单元。
- plugin：openclaw 的 memory plugin，负责自动记忆存储和召回。
- type：记忆垂直类型。
- kinds：横向关联记忆分类。
- scope：记忆范围字段，当前为预留设计。
- state：记忆状态。
- normal mode：普通存储模式。
- smart mode：智能存储模式。
- prebuild mode：客户端注入记忆时的一种模式。
- recall：基于输入内容召回相关记忆。
- fusion：对多路召回结果进行融合。
- rerank：对候选记忆进行精排。

# Architecture

整体架构为客户端 + 服务端。

- 服务端向外提供 memory 管理接口（HTTP API），数据存储在数据库中。还需提供一个 dashboard，用于展示记忆内容。
- 客户端即 memory plugin，负责为 Agent 提供记忆存储和召回能力，并调用服务端接口。

# Server Design

技术栈偏好: Go + Gin + GORM + TiDB + AI SDK

## Purpose

提供 memory 的存储、管理、搜索与召回能力。

## Capabilities

- 向外提供 memory 的增删改查与搜索接口。
  - 创建接口：创建 memory，或在 smart mode 下智能决定 create / update。
  - 查询接口：根据 id 查询 memory。
  - 列表接口：查询 memory 列表，支持分页。
  - 搜索接口：根据关键词搜索相关 memory。
  - 删除接口：删除 memory。
  - 更新接口：更新 memory 内容或状态。
- 内部 memory 存储在数据库中，需向量化。

## API Preference

以下为接口偏好示例：

```http
POST /api/v1/memories
DELETE /api/v1/memories/{id}
GET /api/v1/memories/{id}
PUT /api/v1/memories/{id}
GET /api/v1/memories
GET /api/v1/memories?search=xxx
```

## Memory Model

### Memory Type

`type` 表示记忆垂直类型，仅支持以下几种，可为空：

- `fact`：事实。
- `episodic`：发生过的事。
- `procedural`：流程、经验、约定。

### Memory Kind

`kinds` 表示横向关联记忆，可拓展。一条记忆可同属于多个 `kinds`，可为空。请求输入若只传单个 `kind`，服务端可转换为单元素 `kinds` 处理。包括但不限于：

- `skill`
- `task`
- `lesson`
- `workflow`
- `preference`
- `profile`
- `note`
- `decision`

### Memory Scope

`scope` 表示记忆范围，为保留字段。v1 仅存储，不参与过滤、召回、排序与隔离逻辑，默认值为 `user`。

- `user`：用户记忆。
- `agent`：agent 记忆。
- `external`：外部记忆。

### Memory State

- `creating`：创建中，尚未完成创建，不被查询和使用。
- `active`：可用状态，正常被查询和使用。
- `archived`：归档状态，不被查询和使用，但保留在系统中，可供后续恢复或审计。

状态规则：`creating` 不参与搜索、召回和默认列表查询；`active` 可被搜索、召回和正常使用；`archived` 不参与搜索和召回。

## Database Design

memory 至少包含以下基础字段：`id`、`content`、`embedding`、`type`、`kinds`、`scope`、`state`、`created_at`、`updated_at`、`content_hash`、`version`。

基于最终设计需要，还可包含更多管理字段，如 `store_count`、`use_count`、`last_accessed_at`、`agent_id`、`session_id`、`metadata` 等。他们可以用于任何召回后精排过滤。

以下为示例，不要完全遵循，不要完全遵循，不要完全遵循。

```sql
CREATE TABLE memories (
  id              VARCHAR(36) PRIMARY KEY,
  content         TEXT NOT NULL,
  embedding       VECTOR(1536) NULL,           -- 向量嵌入
  content_hash    VARCHAR(64) NULL,            -- content 的 hash 值，用于去重
  type            VARCHAR(20),                 -- 记忆类型：fact/episodic/procedural
  scope           VARCHAR(20) DEFAULT 'user',  -- 记忆范围：user/agent/external
  kinds           JSON,                        -- 记忆种类列表
  metadata        JSON,                        -- 元数据
  agent_id        VARCHAR(100),                -- Agent ID
  session_id      VARCHAR(100),                -- 会话 ID
  source          VARCHAR(100),                -- 记忆来源，如 plugin、manual 等
  state           VARCHAR(20) DEFAULT 'active',-- creating/active/archived
  version         INT DEFAULT 1,               -- 乐观锁版本
  store_count     INT DEFAULT 0,               -- 存储次数，用于衡量记忆的重要程度
  last_accessed_at TIMESTAMP,                  -- 上次被召回并注入 prompt 的时间
  use_count       INT DEFAULT 0,               -- 使用次数，用于衡量记忆的重要程度
  created_at      TIMESTAMP,
  updated_at      TIMESTAMP,

  INDEX idx_memory_type (type),
  INDEX idx_state (state),
  INDEX idx_agent (agent_id),
  INDEX idx_session (session_id),
  UNIQUE INDEX idx_content_hash (content_hash)
);
```

## embedding

支持多 provider，默认基于 `text-embedding-3-small` 模型，共 1536 维。

## Ingest Design

### Purpose

定义创建记忆时的存储模式与处理流程。

### Modes

创建记忆（存储记忆）时，支持两种模式：

- normal mode：必须提供 `content`，可选提供 `type`、`scope`、`kind`，直接 embedding 完存储。
- smart mode：必须提供 `content`，可选提供 `scope`。细节如下：

使用`信息提取` prompt 基于 LLM 自动提取关键信息。LLM 返回一条或多条原子化的 `content` + `type` + `kind` 组合，最多 5 条，对于每一条记忆：
1. 召回相关记忆，直接调用 Recall 相关方法，召回 5 条记忆。
2. 使用 `记忆融合` prompt 基于 LLM 进行 reconcile。LLM 返回每一条记忆的处理方式：
   1. 对候选记忆只允许三种决策：`ignore`、`create`。
      1. ignore: 无用信息忽略；已存在相关信息，更新老信息。
      2. create: 创建新记忆。
   2. 对老记忆只允许两种决策：`delete`、`update`。
      1. delete: 删除老记忆，一般是记忆冲突。
      2. update: 更新老记忆，更新需要更新的信息，包括  content，content_hash，store_count
3. 最终存储时，进行 embedding 存储，`content_hash` 用于精确去重；创建 SQL 时，需要设计为`content_hash` 冲突则转而更新对应记录的 `store_count`。

异步设计：

异步的操作：设计 ingest 表，用于异步操作。API 插入 ingest 表即返回成功。
1. ingest 表需要定义 content, type, scope, kind , state , created_at, updated_at，mode，execute_count 等信息。要求
   1. 保存完整请求信息
   2. 同时定义状态，表示该数据的处理进度，重启可继续执行
2. 启动一个 worker 去轮询 ingest 表，进行上述 ingest 流程。
3. 具备失败重试，最多 5 次。

原则

1. 智能模式下，希望 LLM 自动提取关键信息时，尽量提取尽量小和原子的内容。
2. 一条 durable memory 应尽量只表达一个事实、偏好、决策或流程要点，不应直接把整轮会话摘要作为单条 memory 存储。
3. `POST /api/v1/memories` 创建接口用于 normal mode 或 smart mode 写入。
4. reconcile 时每条记忆的处理尽量使用事务，让一整次数据库操作在一个事务中。
5. prompt 见下文：Prompt 设计

## Prompt 设计

用于 ingest 过程中与 LLM 的交互

### 信息提取

以下是参考

mem0
```
```

supermemory
```
```

mem9
```
```

clawmem
```
```

### 记忆融合

mem0
```
```

supermemory
```
```

mem9
```
```

clawmem
```
```

## Recall Design

### Purpose

基于给定 `content` 召回相关记忆。

### Inputs

- 输入内容：`content`
- 可配置召回数量：默认召回 top 5 条相关记忆，可配置召回 1-10 条相关记忆。

### Flow

假设要求召回 top k 条相关记忆，召回流程如下：

1. 基于 `content` 粗排。
   1. 基于向量搜索 2k 条记忆，只搜 `active` 状态的记忆，注意需要保留 distance。
   2. 基于全文搜索 2k 条记忆，只搜 `active` 状态的记忆，注意保留 score。
   3. 使用 RRF 融合两种搜索结果，得到最终 top 2k 条记忆。
2. rerank 精排得到 top k。
   1. 打分策略：
      1. 权重打分：基于第一步中的 `distance`, `score`。以及创建时间、最近更新时间、存储次数等方面设置权重进行打分。未来可增加 `type`、`kinds` 等。
      2. reranker 打分：如 `CohereReranker`、`cross-encoder`，暂时不实现，但预留位置。
   2. 对得分进行 softmax，并设置 `Temperature` 参数。得到概率后，按概率召回 topk，而不是直接取前 k 个。

### Relevance-Gated Rerank

`recency`、`store_count` 这类业务信号不能脱离检索相关性单独抬分，否则完全无关但较新、被多次存储的记忆也会获得较高分数，污染召回结果。

> 相关性信号决定“能不能进场”，时间和存储次数只负责在相关候选之间做微调，不负责让弱相关或无关候选翻盘。

1. 先基于 `distance` 和 `score` 计算 `relevance` 主分。
2. 再引入 `recency`、`store_count` 作为 boost。
3. 只有当 `relevance` 超过一个较低门槛后，业务信号才参与增强；当 `relevance` 很低时，直接返回 `relevance`，不再叠加 boost。


4. `relevance = 0.6 * vector_similarity + 0.4 * full_text_score`
5. `vector_similarity` 由 `distance` 归一化得到，`full_text_score` 由当前候选集合中的最大 `score` 归一化得到。
6. 通过门槛后，再计算 `boost = 0.7 * recency + 0.3 * store_count_score`
7. 最终得分为 `relevance + 0.1 * boost`

# Client Plugin Design

## Purpose

针对 openclaw 设计 memory plugin，提供自动记忆存储和召回能力。

## Injection Points

- `agent_end`
  - 每轮用户输入触发一次，不是每会话一次。
  - 需要注意 `agent_end` 还会有前几轮对话内容，设计时需要注意去重，防止重复存储。
  - 格式化信息，然后调用服务端记忆存储接口，存储记忆。
- `before_prompt_build`
  - 提供两种模式：
  - 默认模式：传入 prompt，请求服务端搜索，召回注入到 prompt 中。
  - `prebuild` 模式：除了第一轮对话，直接注入上一轮对话召回的记忆。该模式仅复用相邻轮次的结果；当用户输入主题明显变化时，应放弃复用并重新搜索。

## Key Decision

1. client 去重策略：注入到 prompt 时，用 `memory` 包括召回的记忆内容。然后再 `agent_end` 存储记忆时，去掉召回的 `memory`，防止重复存储。

## Parameters

1. 服务端地址。
2. `EnablePrebuild`：是否启用 `prebuild` 模式。
3. `RecallNum`：召回记忆数量，默认 5 条。

# Dashboard Design

需要提供一个 dashboard，用于展示记忆内容。v1 至少包含以下能力：

- 浏览和搜索 memory，浏览需要一个炫酷的网状图案展示 memory 之间的关联关系。
- 查看 memory 的内容、类型、状态、更新时间、使用次数等元数据。
- 支持手动编辑、删除或归档 memory。

技术栈偏好：：Vite + React 19 + TypeScript + Tailwind CSS 4 + shadcn/ui + TanStack

# Future

暂时不实现，未来实现。

摘要设计：

- v1 不包含摘要设计。后续版本可在 openclaw `session_end`、`reset` 时获取最近 5 轮对话内容，提取摘要，并存储。
