# PowerMem 记忆机制调研

## 概览

PowerMem 的长期记忆核心不只是“向量存储”，而是一个完整的写入与召回管线：

1. 写入时先做事实抽取
2. 再检索相似旧记忆
3. 再让 LLM 决定是新增、更新、删除还是忽略
4. 召回时走 dense vector / full-text / sparse vector 混合检索
5. 再做融合、可选 rerank、可选遗忘曲线生命周期管理

关键源码位置：
- 主入口：`src/powermem/core/memory.py`
- 异步入口：`src/powermem/core/async_memory.py`
- 用户画像与 query rewrite：`src/powermem/user_memory/user_memory.py`
- 事实抽取与记忆更新 prompt：`src/powermem/prompts/intelligent_memory_prompts.py`
- query rewrite prompt：`src/powermem/prompts/query_rewrite_prompts.py`
- OceanBase 混合检索：`src/powermem/storage/oceanbase/oceanbase.py`
- 存储适配层：`src/powermem/storage/adapter.py`
- Ebbinghaus 插件：`src/powermem/intelligence/plugin.py`
- Ebbinghaus 算法：`src/powermem/intelligence/ebbinghaus_algorithm.py`

---

## 一、写入链路：记忆如何提取与融合

### 1.1 入口与触发时机

主入口是 `Memory.add(...)`。

- `infer=False`：直接原文入库，走 `_simple_add(...)`
- `infer=True`：走智能写入，进入 `_intelligent_add(...)`

对应代码：
- `src/powermem/core/memory.py:591`
- `src/powermem/core/memory.py:649-657`
- `src/powermem/core/memory.py:805-817`

也就是说，**事实抽取只发生在 add 时，不发生在 search 时**。

### 1.2 智能写入总流程

`_intelligent_add(...)` 的流程是：

1. 从对话中抽取 facts
2. 为每条 fact 做 embedding
3. 用每条 fact 去检索相似旧记忆
4. 去重并截断候选旧记忆
5. 让 LLM 决定 `ADD / UPDATE / DELETE / NONE`
6. 执行动作

对应代码：`src/powermem/core/memory.py:821-1026`

### 1.3 事实抽取

#### 输入整理

`parse_messages_for_facts(...)` 会把消息转成：

```text
user: ...
assistant: ...
```

并跳过 `system` 消息。

代码：`src/powermem/prompts/intelligent_memory_prompts.py:146-170`

#### 模型调用

`Memory._extract_facts()` 会调用一次 LLM。

代码：`src/powermem/core/memory.py:462-520`

默认使用的 system prompt 是 `FACT_RETRIEVAL_PROMPT`，内容定义在：

`src/powermem/prompts/intelligent_memory_prompts.py:19-57`

这个 prompt 的核心要求：
- 抽用户偏好、身份信息、计划、意图、需求、活动、健康、职业信息
- 强制保留时间信息
- 输出 JSON：`{"facts": [...]}`
- 不翻译，保持原语言

user prompt 很简单：

```text
Input:
{conversation}
```

#### 是否一定调用模型

是。智能写入链路里，事实抽取是显式 LLM 调用。

另外它用了 `llm_json_text_with_fallback(...)`，如果结构化 JSON 返回失败，还会退回普通文本模式再解析一次。

### 1.4 相似旧记忆召回

抽出 facts 后，会对每条 fact：

1. 生成 embedding
2. 调 `storage.search_memories(...)`
3. `limit=5`
4. 把 query 文本也传给存储层，启用 hybrid search

代码：`src/powermem/core/memory.py:838-872`

然后做两层清洗：
- 按 `id` 去重
- 如果同一条记忆从多个 fact 命中，保留距离更小的那条
- 最终最多保留 10 条候选记忆给 LLM

代码：`src/powermem/core/memory.py:874-889`

### 1.5 记忆融合：不是 append，而是 LLM 决策归并

PowerMem 的“融合”发生在 `_decide_memory_actions(...)`。

这里会把：
- 新抽出的 facts
- 检索回来的旧 memories

一起喂给 LLM，让它决定：
- `ADD`
- `UPDATE`
- `DELETE`
- `NONE`

代码：`src/powermem/core/memory.py:522-589`

使用的 prompt 生成函数：
- `get_memory_update_prompt(...)`
- 定义：`src/powermem/prompts/intelligent_memory_prompts.py:98-143`

默认规则包括：
- 新信息不存在于旧记忆 -> `ADD`
- 新信息是旧记忆的增强版 -> `UPDATE`
- 明显矛盾 -> `DELETE`
- 已存在或无关 -> `NONE`
- 对时间信息特别敏感，优先保留更完整、更近、更明确的时间表述

这是 PowerMem 最关键的点之一：

**它不是“把对话原样存下来”，而是“抽事实后和已有记忆做归并”。**

### 1.6 执行动作

LLM 返回动作后：
- `ADD` -> `_create_memory(...)`
- `UPDATE` -> `_update_memory(...)`
- `DELETE` -> 删除对应记忆
- `NONE` -> 跳过

代码：`src/powermem/core/memory.py:919-1026`

### 1.7 简单写入模式

如果 `infer=False`，则不会抽事实，也不会调用 LLM 做归并，只会：

1. 拼接消息内容
2. 生成 embedding
3. 直接入库

代码：`src/powermem/core/memory.py:664-803`

### 1.8 用户画像：UserMemory 的额外一层抽取

`UserMemory.add(...)` 在 `memory.add(...)` 之后，还会额外做用户画像抽取：

1. 先存事件记忆
2. 再取当前用户已有 profile
3. 调 LLM 抽取/更新 profile
4. 写入 `user_profiles` 表

代码：
- `src/powermem/user_memory/user_memory.py:205-256`
- `src/powermem/user_memory/user_memory.py:350-475`

画像 prompt 定义在：
- `src/powermem/prompts/user_profile_prompts.py:99-167`

这层不是核心 memory store 的一部分，但会在后续 query rewrite 中参与召回增强。

---

## 二、召回链路：用了什么搜索、算法和评分

### 2.1 统一入口

主入口：`Memory.search(...)`

流程：

1. 对 query 生成 embedding
2. 调存储层 `search_memories(...)`
3. 可选 intelligence 后处理
4. 可选 Ebbinghaus 生命周期更新
5. 返回结果

代码：`src/powermem/core/memory.py:1193-1365`

### 2.2 StorageAdapter 的角色

`StorageAdapter.search_memories(...)` 是统一桥接层。

它会：
- 合并 `user_id/agent_id/run_id/filters`
- 生成 sparse embedding（若配置了 sparse embedder）
- 把 query text 和 dense embedding 一起传给底层向量库

代码：`src/powermem/storage/adapter.py:130-277`

### 2.3 不同后端的检索能力

#### SQLite

- 纯向量搜索
- 算法：cosine similarity

代码：`src/powermem/storage/sqlite/sqlite_vector_store.py:124-179`

#### PGVector

- 纯向量搜索
- SQL 用 `vector <=> query_vector` 做距离排序

代码：`src/powermem/storage/pgvector/pgvector.py:232-275`

#### OceanBase

OceanBase 是 PowerMem 最完整的召回实现，支持三路检索：

1. dense vector search
2. full-text search
3. sparse vector search

入口：`src/powermem/storage/oceanbase/oceanbase.py:858-870`

### 2.4 Dense vector search

`_vector_search(...)` 使用 `ann_search(...)`，支持：
- `l2`
- `cosine`
- `inner_product`

并统一转换成 0~1 的 similarity，保存在：
- `_vector_similarity`
- `_quality_score`（纯向量时等于 vector similarity）

代码：`src/powermem/storage/oceanbase/oceanbase.py:872-951`

### 2.5 Full-text search

`_fulltext_search(...)` 默认用：

```sql
MATCH(fulltext_content) AGAINST(:query IN NATURAL LANGUAGE MODE)
```

失败时退回 `LIKE` 搜索。

得分保存在 `_fts_score`。

代码：`src/powermem/storage/oceanbase/oceanbase.py:957-1068`

### 2.6 Sparse vector search

`_sparse_search(...)` 用：

```sql
negative_inner_product(sparse_embedding, query_sparse)
```

再转成 0~1 similarity，保存为 `_sparse_similarity`。

代码：`src/powermem/storage/oceanbase/oceanbase.py:1070-1168`

### 2.7 Native hybrid search

如果 OceanBase 版本和表条件满足，还可以走数据库原生：

`DBMS_HYBRID_SEARCH.SEARCH`

代码：`src/powermem/storage/oceanbase/oceanbase.py:1170-1311`

启用条件：
- `enable_native_hybrid=True`
- `threshold is None`
- filter 字段都能映射到表列

否则退回应用层 hybrid search。

### 2.8 应用层 hybrid search

`_hybrid_search(...)` 的逻辑：

1. 并行跑 vector / fts / sparse 三路召回
2. 合并候选
3. 做 coarse ranking
4. 可选再做 rerank

代码：`src/powermem/storage/oceanbase/oceanbase.py:1312-1445`

如果是嵌入式 SeekDB，因线程安全限制，三路检索顺序执行；如果是远程 OceanBase，则用线程池并行检索。

### 2.9 融合算法

支持两种：
- 默认 `rrf`
- 备选 `weighted`

代码：`src/powermem/storage/oceanbase/oceanbase.py:1565-1575`

#### RRF

RRF 分数形态是：

```text
weight * 1 / (k + rank)
```

默认 `k=60`。

代码：`src/powermem/storage/oceanbase/oceanbase.py:1629-1754`

#### Weighted fusion

直接线性加权：

```text
combined_score = vector_w * vector_score + fts_w * fts_score + sparse_w * sparse_score
```

代码：`src/powermem/storage/oceanbase/oceanbase.py:1756-1868`

### 2.10 评分体系：排序分和质量分分离

PowerMem 里有两个不同的分数概念。

#### `score`

这是最终对外返回、用于排序的分数。

它可能是：
- 纯向量 similarity
- RRF 融合分
- weighted fusion 分
- rerank 分

#### `_quality_score`

这是用于 `threshold` 过滤的绝对质量分。

它不是 RRF，而是“参与通道的 similarity 加权平均”：

代码：`src/powermem/storage/oceanbase/oceanbase.py:1500-1564`

设计目的很明确：

- `score` 适合排序
- `_quality_score` 适合做阈值过滤

因为 RRF 这种 rank-based 分数不适合作绝对阈值。

`Memory.search(...)` 过滤时优先用 `_quality_score`：

代码：`src/powermem/core/memory.py:1283-1297`

### 2.11 Rerank

OceanBase 混合召回后可选使用 reranker 做 fine ranking。

初始化位置：
- `src/powermem/core/memory.py:191-214`

应用位置：
- `src/powermem/storage/oceanbase/oceanbase.py:1433-1498`

例如 Qwen rerank：
- `src/powermem/integrations/rerank/qwen.py:54-129`

注意：rerank 后 `result.score` 会被改成 rerank score，而原始融合分保留在 `_fusion_score`。

---

## 三、RRF 自适应权重归一化：怎么做的

这是 PowerMem 在召回融合上一个很值得关注的工程细节。

### 3.1 要解决的问题

普通加权 RRF 会有一个不公平点：

- 有些文档同时命中 vector + fts + sparse
- 有些文档只命中 vector + fts
- 还有些文档只命中 vector

如果直接用固定权重相加，那么“命中通道更多”的文档天然更容易拿到更高分。

但这不一定代表它语义上更相关，有时只是因为：
- 历史数据里并不是每条文档都有 sparse vector
- 某一路检索本身对某类文档更友好

所以 PowerMem 加了“按文档参与的实际通道数重新归一化”的逻辑。

对应实现：
- `src/powermem/storage/oceanbase/oceanbase.py:1577-1627`

### 3.2 具体算法

假设系统全局权重是：

- `vector_w = 0.5`
- `fts_w = 0.3`
- `sparse_w = 0.2`

对于某一条文档：

1. 先看它实际命中了哪些通道
2. 把这些通道的权重取出来
3. 对这些“有效权重”重新归一化到总和 1
4. 再计算这条文档自己的 RRF 分

代码里的核心是：

```text
normalized_weight = weight / total_weight
normalized_score += normalized_weight * (1 / (k + rank))
```

其中：
- `total_weight` 只统计这条文档实际参与的通道
- 没命中的通道不参与分母

### 3.3 一个例子

假设：
- 全局权重还是 `0.5 / 0.3 / 0.2`
- `k=60`

#### 文档 A：命中 vector + fts + sparse

总有效权重：

```text
0.5 + 0.3 + 0.2 = 1.0
```

归一化后还是：
- vector: 0.5
- fts: 0.3
- sparse: 0.2

#### 文档 B：只命中 vector + fts

总有效权重：

```text
0.5 + 0.3 = 0.8
```

归一化后变成：
- vector: 0.5 / 0.8 = 0.625
- fts: 0.3 / 0.8 = 0.375

也就是说，文档 B 不会因为“没 sparse 命中”而平白损失那 0.2 的权重。

#### 文档 C：只命中 vector

总有效权重：

```text
0.5
```

归一化后：
- vector: 1.0

它的 RRF 分就是纯 vector 的 rank 分，不会被不存在的 FTS / sparse 通道压低。

### 3.4 代码层面的数据结构

在 `_rrf_fusion(...)` 里，每条文档会记录：
- `vector_rank`
- `fts_rank`
- `sparse_rank`
- `rrf_score`

然后 `_normalize_weights_adaptively(...)` 会对每个 `doc_data`：
- 收集非空 rank 的通道
- 求这些通道的总权重
- 重算 `rrf_score`

代码：
- `src/powermem/storage/oceanbase/oceanbase.py:1647-1702`

### 3.5 这样做的意义

它解决的是“混合检索数据并不对称”时的公平性问题：

- 历史数据未迁移 sparse vector 时，不会系统性吃亏
- 某些文档只适合 lexical 命中，也不会因为没 semantic hit 就被压得太低
- 不同召回路径覆盖率不一致时，排序更稳健

这是一个明显偏工程实践的改进点，不是理论新算法，但很实用。

### 3.6 它和 `_quality_score` 的关系

这两个东西不要混：

- `rrf_score`：排序分，rank-based
- `_quality_score`：质量分，similarity-based

RRF 自适应权重归一化只影响排序，不负责阈值过滤。

阈值过滤仍然用 `_quality_score`。

---

## 四、用户画像与 Query Rewrite

`UserMemory.search(...)` 在检索前还可以做一层 query rewrite。

流程：

1. 先从 `user_profiles` 取出该用户的 `profile_content`
2. 用画像 + 原 query 构造 rewrite prompt
3. 调 LLM 生成更明确的 query
4. 再把改写后的 query 送进 `memory.search(...)`

代码：
- `src/powermem/user_memory/user_memory.py:500-527`
- `src/powermem/user_memory/query_rewrite/rewriter.py:44-118`

prompt 定义：
- `src/powermem/prompts/query_rewrite_prompts.py:3-46`

默认规则很克制：
- 用用户画像补全模糊指代
- 保持原意不变
- 如果 query 已经很明确，就不要改

这意味着 PowerMem 在召回前有两层增强：

1. 检索层 hybrid search
2. 检索前 query rewrite

---

## 五、Ebbinghaus 生命周期管理

### 5.1 插件化接入点

PowerMem 把“长期记忆生命周期”做成了插件接口：

- `on_add(...)`
- `on_get(...)`
- `on_search(...)`

接口和默认实现：
- `src/powermem/intelligence/plugin.py:19-257`

默认插件是 `EbbinghausIntelligencePlugin`。

### 5.2 add 时做什么

在 `on_add(...)` 中，它会：

1. 评估 importance
2. 分类 memory type：`working / short_term / long_term`
3. 生成 Ebbinghaus 相关 metadata

包括：
- `importance_score`
- `memory_type`
- `review_schedule`
- `current_retention`
- `should_promote / should_forget / should_archive`

代码：`src/powermem/intelligence/plugin.py:95-119`

### 5.3 get/search 时做什么

`on_get(...)` / `on_search(...)` 会：

- 更新 access_count
- 判断是否该 forget
- 判断是否该 promote
- 判断是否该 archive
- 定期重新计算 intelligence metadata

代码：
- `src/powermem/intelligence/plugin.py:121-257`

### 5.4 当前实现的现实情况

虽然代码里有 `ImportanceEvaluator`，而且支持 LLM-based importance evaluation，但当前主链路里，LLM importance 打分基本是关闭/弱化的。

原因：
- `Memory.add()` 中原本 `self.intelligence.process_metadata(...)` 被注释掉了，注释明确写着“Disabled LLM-based importance evaluation to save tokens”

代码：
- `src/powermem/core/memory.py:714-717`
- `src/powermem/core/memory.py:1094-1097`

另外默认 Ebbinghaus plugin 初始化 `ImportanceEvaluator` 时，也没有看到给它注入可用的 LLM 实例；因此更可能走 rule-based fallback。

所以更准确的说法是：

- **事实抽取、记忆归并、用户画像、query rewrite 是强 LLM 驱动的**
- **importance / forgetting 生命周期这条线，设计上支持 LLM，但当前实现更偏规则化和省 token**

---

## 六、与 OpenClaw 如何集成，注入点是什么

### 6.1 本仓库能确认的事实

本仓库没有直接包含 OpenClaw 插件实现。

README 里的说法是：

- OpenClaw 通过外部插件 `memory-powermem` 把 PowerMem 当作长期记忆使用

见：
- `README_CN.md:18-26`

所以：

- OpenClaw 的真正适配层不在 `src/powermem/...`
- 它在外部插件仓库 `memory-powermem`

### 6.2 从 PowerMem 视角看，可被外部系统接入的注入面

PowerMem 暴露了三类典型能力：

1. Python SDK：`Memory` / `UserMemory`
2. HTTP API：`/api/v1/memories`、`/api/v1/memories/search`
3. MCP Server

因此 OpenClaw 若集成 PowerMem，合理的注入点通常是：

#### 写入注入点

- 对话结束
- compact 后
- 某个记忆 flush 时机
- 显式 remember tool

最终调用 PowerMem 的：
- `add(...)`
- 或 HTTP `POST /api/v1/memories`
- 或 MCP `add_memory`

#### 召回注入点

- system prompt 组装前
- user prompt submit 时
- LLM 显式 tool call

最终调用 PowerMem 的：
- `search(...)`
- 或 HTTP `POST /api/v1/memories/search`
- 或 MCP `search_memories`

### 6.3 仓库里能参考的相似实现

`apps/claude-code-plugin` 不是 OpenClaw，但它清楚展示了这种外部集成方式：

- `UserPromptSubmit` 时调用 `POST /api/v1/memories/search`，把结果注入 `additionalContext`
- `SessionEnd` / `PostCompact` 时调用 `POST /api/v1/memories` 写入长期记忆
- MCP mode 下显式暴露 `search_memories` / `add_memory`

见：
- `apps/claude-code-plugin/README.md:7-13`
- `apps/claude-code-plugin/README.md:163-185`
- `apps/claude-code-plugin/README.md:223-227`

所以如果问“PowerMem 与 OpenClaw 的注入点是什么”，基于本仓库能落到实处的回答是：

- **写入注入点**：`add` 能力
- **召回注入点**：`search` 能力
- **具体接到 OpenClaw 的哪一个 lifecycle / slot / hook**：不在本仓库，需要去看 `memory-powermem` 插件实现

---

## 七、这个仓库在长期记忆上的特色点

### 7.1 检索增强的写入归并

不是简单 append 对话，而是：

1. 抽 facts
2. 召回旧记忆
3. LLM 决策是否合并

这比“直接存 transcript”更像长期记忆系统。

### 7.2 混合检索路径很完整

同时支持：
- dense vector
- FTS
- sparse vector
- rerank
- native hybrid search

很多长期记忆系统只有单路向量召回，这里明显更完整。

### 7.3 排序分与质量分分离

这是很实用的工程设计：

- `score` 用于排序
- `_quality_score` 用于 threshold

避免了 rank-based 分数被误拿去做绝对过滤。

### 7.4 RRF 自适应权重归一化

它不算理论新算法，但在混合检索场景里很有价值。

特别是：
- 历史数据是否已补 sparse
- 某些文档是否命中全部通道
- 不同检索路径覆盖率不一致

这种现实问题下，它能明显减少排序偏差。

### 7.5 用户画像驱动 query rewrite

它不只是“搜记忆”，还先用用户画像把 query 改写得更明确。

这是长期记忆系统里比较常见但并不总是做好的增强点，这个仓库已经落地了。

### 7.6 Ebbinghaus 生命周期插件化

它把：
- importance
- memory type
- review schedule
- forget / promote / archive

做成了独立插件接口，而不是写死在检索逻辑里。这使得后续替换生命周期算法比较容易。

---

## 八、结论

PowerMem 的核心特点可以概括成一句话：

**写入时做“事实抽取 + 旧记忆归并”，召回时做“多路混合检索 + 融合排序 + 生命周期增强”。**

如果只看最重要的两个点：

1. 它的长期记忆不是 append-only，而是 LLM 驱动的增量归并
2. 它的召回不是单路向量搜索，而是 dense/fts/sparse/rerank 的完整混合检索栈

而你特别关心的 RRF 自适应权重归一化，本质上是在解决：

**同一批候选文档命中通道数不一致时，如何避免“命中通道更多”的文档天然占优。**

它的办法非常直接：

- 对每条文档，只对它实际命中的通道做权重归一化
- 再计算 RRF 分

这让混合检索在真实脏数据、部分迁移数据、覆盖率不一致的数据集上更稳。
