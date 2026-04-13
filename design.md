# Design

## Overview

`smem` 的目标是实现一个用于 Agent 的记忆系统。

## Goals

1. 可用 demo：可用，但质量要高于 demo。实现 openclaw plugin + TiDB Cloud 存储。
2. 个人控制：仅用于个人搭建，不做服务。用户对数据完全控制，用户提供必要执行环境，如数据库、LLM、Agent 运行环境等。
3. 创新：本项目用于个人学习，需要对比市面常见记忆系统，需要有一定创新点/难点。

## Non-goals

- 服务化：包括权限、服务端全托管等。
- 其他 agent 插件、其他存储。

## Constraints

- 服务端本地运行: 用户需自行提供 LLM API key 和数据库连接信息。服务端使用 Go 实现。
- 客户端: openclaw 设计 memory plugin，使用 TypeScript 实现。

## Glossary

- memory：系统中的记忆单元。
- plugin：openclaw 的 memory plugin，负责自动记忆存储和召回。
- type：记忆垂直类型。
- kind：横向关联记忆分类。
- scope：记忆范围字段，当前为预留设计。
- state：记忆状态。
- normal mode：普通存储模式。
- smart mode：智能存储模式。
- prebuild mode：客户端注入记忆时的一种模式。
- recall：基于输入内容召回相关记忆。
- fusion：对多路召回结果进行融合。
- rerank：对候选记忆进行精排。

## Architecture

整体架构为客户端 + 服务端。

- 服务端向外提供 memory 管理接口（HTTP API），数据存储在数据库中。还需提供一个 dashboard，用于展示记忆内容。
- 客户端即 memory plugin，负责为 Agent 提供记忆存储和召回能力，并调用服务端接口。

## Server Design

### Purpose

提供 memory 的存储、管理、搜索与召回能力。

### Capabilities

- 向外提供 memory 的增删改查与搜索接口。
  - 创建接口：存储 memory，可能覆盖原有 memory。
  - 查询接口：根据 id 查询 memory。
  - 列表接口：查询 memory 列表，支持分页。
  - 搜索接口：根据关键词搜索相关 memory。
  - 删除接口：删除 memory。
  - 更新接口：更新 memory 内容或状态。
- 内部 memory 存储在数据库中，需向量化。

### API Preference

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

`kind` 表示横向关联记忆，可拓展。一条记忆可同属于多个 `kind`，可为空。包括但不限于：

- `skill`
- `task`
- `lesson`
- `workflow`
- `preference`
- `profile`
- `note`

### Memory Scope

`scope` 表示记忆范围，为预留字段。先不实现，默认为全 user。

- `user`：用户记忆。
- `agent`：agent 记忆。
- `external`：外部记忆。

### Memory State

- `creating`：创建中，尚未完成创建，不被查询和使用。
- `active`：可用状态，正常被查询和使用。
- `archived`：归档状态，不被查询和使用，但保留在系统中，可供后续恢复或审计。

### Database Design

memory 至少包含以下基础字段：`id`、`content`、`embedding`、`type`、`kind`、`scope`、`state`、`created_at`、`updated_at`。

基于最终设计需要，还可包含更多管理字段，如 `hash`、`agent_id`、`session_id`、`version`、`metadata` 等。`metadata` 可用于任何召回后的过滤。

以下为示例，不一定要完全遵循：

```sql
CREATE TABLE memories (
  id              VARCHAR(36) PRIMARY KEY,
  content         TEXT NOT NULL,
  embedding       VECTOR(1536) NULL,           -- 向量嵌入
  hash            VARCHAR(64) NULL,            -- content 的 hash 值，用于去重
  type            VARCHAR(20),                 -- 记忆类型：fact/episodic/procedural
  scope           VARCHAR(20) DEFAULT 'user',  -- 记忆范围：user/agent/external
  kind            VARCHAR(50),                 -- 主记忆种类：skill/task/lesson/workflow/preference/profile/note
  kinds           JSON,                        -- 记忆种类列表
  metadata        JSON,                        -- 元数据
  agent_id        VARCHAR(100),                -- Agent ID
  session_id      VARCHAR(100),                -- 会话 ID
  state           VARCHAR(20) DEFAULT 'active',-- active/archived
  version         INT DEFAULT 1,               -- 乐观锁版本
  created_at      TIMESTAMP,
  updated_at      TIMESTAMP,

  INDEX idx_memory_type (type),
  INDEX idx_state (state),
  INDEX idx_agent (agent_id),
  INDEX idx_session (session_id)
);
```

## Storage Design

### Purpose

定义创建记忆时的存储模式与处理流程。

### Modes

创建记忆（存储记忆）时，支持两种模式：

1. normal mode：必须提供 `content`，可选提供 `type`、`scope`、`kind`。直接 embedding 完存储。
2. smart mode：必须提供 `content`，可选提供 `scope`。
   1. 使用 prompt 基于 LLM 自动提取关键信息，生成 `content2`、`type`、`kind`。
   2. 召回相关记忆，使用 prompt 基于 LLM 进行记忆融合，判断是忽略记忆、创建新记忆，还是更新老记忆。
   3. 最终存储时，进行 embedding 存储。

### Key Decision

1. 异步存储：存储为 `creating` 状态即返回成功，直到完成 embedding 后更新为 `active` 状态。

## Recall Design

### Purpose

基于给定 `content` 召回相关记忆。

### Inputs

- 输入内容：`content1`
- 可配置召回数量：默认召回 top 5 条相关记忆，可配置召回 1-10 条相关记忆。

### Flow

假设要求召回 top k 条相关记忆，召回流程如下：

1. 对 `content1` 使用 prompt 基于 LLM 自动提取关键信息，生成 `content2`、`type`、`kind`。
2. 基于 `content2` 粗排。
   1. 基于向量搜索 2k 条记忆，只搜 `active` 状态的记忆。
   2. 基于全文搜索 2k 条记忆，只搜 `active` 状态的记忆。
   3. 使用 RRF 融合两种搜索结果，得到最终 2k 到 4k 条记忆。
3. rerank 精排。
   1. 打分策略：
      1. 基于 `kind` 匹配度、最近更新时间等方面设置权重进行打分。
      2. 暂不实现：可插拔 reranker，如 `CohereReranker`、`cross-encoder`。
   2. 对得分进行 softmax，按概率召回，并设置 `Temperature` 参数，让记忆更发散。

### Key Decision

1. embedding 默认基于 `text-embedding-3-small` 模型，共 1536 维。

## Client Plugin Design

### Purpose

针对 openclaw 设计 memory plugin，提供自动记忆存储和召回能力。

### Injection Points

- `agent_end`
  - 每轮用户输入触发一次，不是每会话一次。
  - 需要注意 `agent_end` 还会有前几轮对话内容，设计时需要注意去重，防止重复存储。
  - 格式化信息，然后调用服务端记忆存储接口，存储记忆。
- `session_end`、`reset`
  - 获取最近 5 轮对话内容，提取摘要。
- `before_prompt_build`
  - 提供两种模式：
  - 默认模式：传入 prompt，请求服务端搜索，召回注入到 prompt 中。
  - `prebuild` 模式：除了第一轮对话，直接注入上一轮对话召回的记忆。

### Key Decision

1. 注入到 prompt 时，用 `memory` 包括召回的记忆内容。然后再 `agent_end` 存储记忆时，去掉召回的 `memory`，防止重复存储。

### Parameters

1. 服务端地址。
2. `EnablePrebuild`：是否启用 `prebuild` 模式。
3. `RecallNum`：召回记忆数量，默认 5 条。

## Dashboard Design

需要提供一个 dashboard，用于展示记忆内容，界面可以更 fancy。

## References

- BM25 是什么：全文检索。
- digest 不参与召回：向量搜索，只能搜索后过滤。
