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

使用`记忆提取` prompt 基于 LLM 自动提取关键信息。LLM 返回一条或多条原子化的 `content` + `type` + `kinds` 组合，称为候选记忆，最多 5 条。
1. 对于每一条候选记忆，召回相关记忆，直接复用 Recall 相关方法。每个召回最多3条记忆，因此一共最多召回 15 条记忆。
2. 使用 `记忆融合` prompt 基于 LLM 进行 reconcile。输入召回的老记忆，以及提取的候选记忆。LLM 返回会每最终记忆的处理方式：
   1. 对候选记忆只允许两种决策：`ignore`、`create`。
      1. ignore: 无用信息忽略；已存在相关信息，更新老信息。
      2. create: 创建新记忆。
   2. 对老记忆只允许三种决策：`delete`、`update`、`ignore`。
      1. delete: 删除老记忆，一般是记忆冲突。
      2. update: 更新老记忆，更新需要更新的信息包括 content。特别重要：content 内容不变也视为更新。如老记忆和候选记忆相同的情况，老记忆 content 不变也视为更新。
      3. ignore: 不做任何操作。
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

目标：将 smart mode 输入的 `content` 提取为最多 5 条候选记忆，每条候选记忆只包含一个原子化信息点，输出结构为 `content` + `type` + `kinds`。

设计原则：

1. 参考 `ref.md` 中 mem9 的原子化、同语言、过滤 query intent 的思路，但收敛为本项目需要的固定字段，不引入 tags、fact_type 等额外结构。
2. 参考 mem0 的“尽量返回可复用事实”思路，但不做泛化的 personal profile 抽取，而是面向本项目 durable memory 的事实、偏好、决策、流程经验。
3. 输出必须足够稳定，方便后续 Recall + Fusion 使用，因此要求严格 JSON、固定枚举、最多 5 条。

#### system prompt

```text
You extract candidate memories from text.

Your job is to turn the input text into up to 5 candidate memories for long-term storage.
Each candidate memory must be small, atomic, and independently retrievable.

## What to return

Return a JSON object with a `memories` array. Each item may contain only:

- `content`: the final memory text
- `type`: one of `fact`, `episodic`, `procedural`, or `""`
- `kinds`: an array of zero or more values from `skill`, `task`, `lesson`, `workflow`, `preference`, `profile`, `note`, `decision`

## Extraction policy

1. Extract only information supported by the input. Do not infer missing facts.
2. Keep memories durable and reusable. Good candidates include facts, preferences, decisions, working rules, plans, lessons, and meaningful background context.
3. Prefer one memory per idea. Do not collapse a whole conversation into one broad summary.
4. If two details are tightly coupled and splitting them would lose meaning, keep them in the same memory.
5. Prefer specific statements over vague summaries.
6. Ignore greetings, filler, transient chatter, and other content with no lasting value.
7. Do not store pure lookup or search intent such as "what is X" or "how do I do Y". If such a request also reveals stable background context, extract only the background context.
8. Preserve the original language of the input. Do not translate.
9. Preserve temporal wording as written. Keep expressions like "tomorrow" or "next week" instead of resolving them to calendar dates.
10. Make each memory self-contained. Avoid unclear pronouns when the referent is obvious from the input.
11. Remove duplicates within the candidate set. If two candidates overlap, keep the more specific one.
12. Return at most 5 memories. If nothing is worth storing, return an empty array.

## Type guidance

- `fact`: stable facts, profile data, background information, objective preferences
- `episodic`: events, experiences, or time-anchored happenings
- `procedural`: workflows, habits, operating rules, learned practices, or how-to knowledge

## Kind guidance

- `skill`: an ability or area of expertise
- `task`: an active task, TODO, or ongoing piece of work
- `lesson`: a lesson learned or postmortem-style takeaway
- `workflow`: a repeatable process or operating sequence
- `preference`: a preference, dislike, or setting choice
- `profile`: identity, role, relationship, or background information
- `note`: important context that does not fit the other kinds well
- `decision`: a chosen direction, conclusion, or explicit decision

`type` may be `""` when no type fits confidently.
`kinds` may be `[]` when no kind fits confidently.
`kinds` may contain multiple values when a memory clearly belongs to more than one category.

## Example

Input:
"I prefer using Go for backend services. I decided that new internal APIs should stay REST-first. I need to revisit the onboarding flow next week."

Output:
{
  "memories": [
    {
      "content": "Prefers using Go for backend services",
      "type": "fact",
      "kinds": ["preference"]
    },
    {
      "content": "Decided that new internal APIs should stay REST-first",
      "type": "procedural",
      "kinds": ["decision", "workflow"]
    },
    {
      "content": "Needs to revisit the onboarding flow next week",
      "type": "",
      "kinds": []
    }
  ]
}

## Output rules

Return valid JSON only. No markdown. No explanation.

{
  "memories": [
    {
      "content": "...",
      "type": "fact",
      "kinds": ["preference"]
    }
  ]
}
```

#### user prompt

```text
Extract candidate memories from the following input.

Requirements:
- Return at most 5 memories
- Keep each memory as atomic as possible
- Preserve the original language
- Return JSON only
- If nothing should be stored, return `{"memories":[]}`

Input:
{{content}}
```

### 记忆融合

目标：将“候选记忆”与“召回的老记忆”进行 reconcile，输出最终动作。候选记忆只允许 `ignore` / `create`；老记忆只允许 `delete` / `update`。

设计原则：

1. 参考 `ref.md` 中 mem9 的“按事实与已有记忆对齐后再决策”的思路，但动作集合改为完全符合本项目 design 的不对称规则。
2. 不采用 mem0 / mem9 的 `ADD` / `NOOP` 全量内存输出，而是只输出需要执行的动作，减少实现歧义。
3. 特别强化本项目的关键约束：候选记忆与老记忆语义相同，也必须把老记忆视为 `update`，即使 `content` 最终不变。

#### system prompt

```text
You reconcile candidate memories against existing memories.

Your job is compare candidate memories with recalled existing memories and decide the final action for each one.

## Inputs

You will receive:

1. `candidate_memories`: extracted candidates, each with `id` and `content`
2. `recalled_memories`: relevant existing memories for those candidates; each recalled memory includes at least `id` and `content`

## Allowed actions

- Candidate memories: `ignore`, `create`
- Existing memories: `update`, `delete`, `ignore`
- Do not output any other action.

## Decision policy

1. If a candidate says the same thing as an existing memory, or is a refinement, normalization, clarification, or extension of it, use `ignore` for the candidate and `update` for that memory. Important: an existing memory must still be `update` even if its final `content` does not change. Reaffirmed or absorbed memories still count as updates.
2. Use `create` only when the candidate cannot be cleanly absorbed by any existing memory.
3. Use `delete` only when an existing memory is clearly contradicted or should be replaced rather than updated. Do not delete a memory merely because it is shorter, older, or less specific.
4. Use `ignore` only when a recalled memory should be fully left alone in this reconciliation. If the candidate reinforces, deepens, clarifies, or otherwise gets absorbed into that memory, use `update` instead. Use `ignore` only for memories that are not the right update target, do not conflict with any candidate, and should receive no content change or reinforcement at all.
5. Avoid duplicate creates. Preserve the original language. Do not translate or invent facts not grounded in the input.
6. Return exactly one action for each unique candidate id and each unique recalled memory id. 
7. Return actions in a stable order: all candidate actions first in candidate input order, then all memory actions in first-seen memory id order.

## Output shape

Return a single `actions` array. Each action must include:

- `target`: `candidate` or `memory`
- `id`: the id of that candidate or existing memory
- `action`: one allowed action for that target

For every `create` or `update`, include:

- `memory.content`: final memory text

For `candidate` + `ignore`, you may include `absorbed_by_memory_ids`. This optional field is an array of memory ids. Use `[]` when no absorbed-by memory can be identified confidently. In most cases it should contain only one id, but it may contain multiple ids when the candidate is absorbed across multiple existing memories.

For `delete` or `ignore`, do not include `memory`.

## Output rules

Return valid JSON only. No markdown. No explanation.

{
  "actions": [
    {
      "target": "candidate",
      "id": "1",
      "action": "ignore",
      "absorbed_by_memory_ids": ["mem_123"]
    },
    {
      "target": "candidate",
      "id": "2",
      "action": "create",
      "memory": {
        "content": "..."
      }
    },
    {
      "target": "memory",
      "id": "mem_123",
      "action": "update",
      "memory": {
        "content": "..."
      }
    },
    {
      "target": "memory",
      "id": "mem_456",
      "action": "delete"
    },
    {
      "target": "memory",
      "id": "mem_789",
      "action": "ignore"
    }
  ]
}

## Examples

Example 1 - create new information

Candidate memories:
[{"id":"c1","content":"Sarah lives in Osaka"}]

Recalled existing memories:
[{"id":"m1","content":"Sarah is my sister"},{"id":"m2","content":"Is a software engineer"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"create","memory":{"content":"Sarah lives in Osaka"}},
    {"target":"memory","id":"m1","action":"ignore"},
    {"target":"memory","id":"m2","action":"ignore"}
  ]
}

Example 2 - create the replacement and delete contradicted information:

Candidate memories:
[{"id":"c1","content":"Dislikes cheese pizza"}]

Recalled existing memories:
[{"id":"m1","content":"Name is John"},{"id":"m2","content":"Loves cheese pizza"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"create","memory":{"content":"Dislikes cheese pizza"}},
    {"target":"memory","id":"m1","action":"ignore"},
    {"target":"memory","id":"m2","action":"delete"}
  ]
}

Example 3 - update an existing memory with richer detail:

Candidate memories:
[{"id":"c1","content":"Loves to play cricket with friends"}]

Recalled existing memories:
[{"id":"m1","content":"User likes to play cricket"},{"id":"m2","content":"User is a software engineer"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"ignore","absorbed_by_memory_ids":["m1"]},
    {"target":"memory","id":"m1","action":"update","memory":{"content":"Loves to play cricket with friends"}},
    {"target":"memory","id":"m2","action":"ignore"}
  ]
}

Example 4 - same information or slight paraphrase still requires memory update:

Candidate memories:
[{"id":"c1","content":"Name is John"},{"id":"c2","content":"Enjoys coffee"}]

Recalled existing memories:
[{"id":"m1","content":"Name is John"},{"id":"m2","content":"Likes coffee"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"ignore","absorbed_by_memory_ids":["m1"]},
    {"target":"memory","id":"m1","action":"update","memory":{"content":"Name is John"}},
    {"target":"candidate","id":"c2","action":"ignore","absorbed_by_memory_ids":["m2"]},
    {"target":"memory","id":"m2","action":"update","memory":{"content":"Likes coffee"}}
  ]
}

Example 5 - age may help choose an update target when there is a true conflict:

Candidate memories:
[{"id":"c1","content":"Prefers VS Code"},{"id":"c2","content":"Works at company Y"}]

Recalled existing memories:
[{"id":"m1","content":"Prefers vim"},{"id":"m2","content":"Works at startup X"}]

Result:
{
  "actions": [
    {"target":"candidate","id":"c1","action":"ignore","absorbed_by_memory_ids":["m1"]},
    {"target":"candidate","id":"c2","action":"ignore","absorbed_by_memory_ids":["m2"]},
    {"target":"memory","id":"m1","action":"update","memory":{"content":"Prefers VS Code"}},
    {"target":"memory","id":"m2","action":"update","memory":{"content":"Works at company Y"}}
  ]
}

```

#### user prompt

```text
Reconcile the candidate memories against the recalled existing memories.

Requirements:
- Follow the system rules
- Return exactly one action for each candidate id and each unique recalled memory id
- Return JSON only

Candidate memories:
{{candidate_memories}}

Recalled existing memories:
{{recalled_memories}}
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