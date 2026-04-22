# OpenClaw Memory Core — 召回与存储架构

> commit: 81ca7bc40b09dbb6386fc5c1cecf237c5f11004a

## 一、Agent Tool 驱动（主动召回）

Agent 拥有两个记忆工具，由 LLM 自主决定何时调用：

### memory_search — 语义搜索

在 MEMORY.md、memory/*.md、以及可选的 session transcripts 中做语义搜索。支持按 `corpus` 参数扩展搜索范围到第三方插件注册的附加语料（如 wiki）。

核心思想：**让 LLM 自己判断何时需要回忆**，而非系统自动注入上下文。每次搜索完成后，系统会异步记录每个命中文本的召回频次、查询来源、概念标签——这些追踪数据是后续 Dreaming 晋升的基础。

### memory_get — 精确读取

当搜索结果指向某个具体文件时，按路径和行号精确读取该片段。相当于搜索是"找到哪本书"，读取是"翻开那一页"。

## 二、System Prompt 注入（被动引导）

单纯提供工具不够——LLM 不会自觉使用。因此在 system prompt 中注入一段 **Memory Recall** 指令：

```
## Memory Recall
Before answering anything about prior work, decisions, dates, people, preferences, or todos:
run memory_search on MEMORY.md + memory/*.md + indexed session transcripts;
then use memory_get to pull only the needed lines.
If low confidence after search, say you checked.
Citations: include Source: <path#line> when it helps the user verify memory snippets.
```

设计思路：
- **无条件触发**：不依赖 LLM 的"自觉"，而是明确指令"回答任何关于过往的问题之前，必须先搜索"
- **置信度兜底**：搜索后仍不确定时，要求声明"我查过了但不确定"，避免幻觉
- **Citation 可控**：通过 `citationsMode` 配置决定是否在回复中附带来源引用
- **动态适配**：根据当前 agent 实际拥有的工具集合调整措辞（只有 search、只有 get、或两者兼有）
- **可扩展**：其他插件可以追加额外的 prompt 段（如 wiki 搜索指引）

## 三、搜索管线

```
Query → Embed → Vector Search ──┐
                    FTS Search ──┤→ Hybrid Merge → Temporal Decay → MMR → Score Filter → Results
```

10 个步骤，从粗到细逐步收敛：

1. **Bootstrap sync**：索引为空时先做一次全量同步，保证有数据可搜
2. **Preflight check**：跳过空查询和无效输入
3. **Async background sync**：检测到文件变更时，非阻塞地在后台触发重索引，不阻塞当前搜索
4. **Provider init**：延迟初始化 embedding 模型，避免冷启动开销
5. **Vector search**：将查询文本 embed 成向量，通过 sqlite-vec 做余弦相似度检索
6. **Keyword search**：同时用 FTS5 BM25 做关键词排序，捕获精确术语匹配
7. **Hybrid merge**：向量结果和关键词结果加权融合（默认 0.7 vector + 0.3 text），兼顾语义理解和精确匹配
8. **Temporal decay**：对融合后的分数施以时间衰减（详见下方）
9. **MMR reranking**（可选）：用 Maximal Marginal Relevance 去重，确保结果多样性，避免返回多条相似内容
10. **Score filter**：按最低分数阈值和最大结果数截断

**降级策略**：当没有 embedding provider 可用时，自动降级为纯关键词搜索（FTS-only），保证记忆功能始终可用。

### Temporal Decay

核心思想：**近期发生的事比很久以前的事更重要**，但不是一刀切——而是平滑衰减。

**公式**：`decayedScore = score × exp(-λ × ageInDays)`，其中 `λ = ln(2) / halfLifeDays`

- 默认半衰期 30 天：30 天前的记忆分数减半，60 天前的减到 1/4
- ageInDays = 0 时衰减因子为 1（无衰减），随时间指数递减

**时间戳来源**（优先级从高到低）：

1. **文件名中的日期**：`memory/2025-04-22.md` 直接从路径解析
2. **常青记忆豁免**：`MEMORY.md` 等非日期命名的记忆文件被视为持久知识，**不参与衰减**
3. **文件修改时间**：其他文件（如 session transcripts）使用 mtime

常青记忆的设计意图：`MEMORY.md` 存放的是经过 Deep 阶段晋升确认的长期知识，不应该因为"旧"就被降权。

## 四、Dreaming（后台巩固）

灵感来自人类睡眠的三个阶段，通过 cron 定时任务在后台运行：

| 阶段 | 调度 | 类比 | 做什么 |
|------|------|------|--------|
| **Light** | 每 6 小时 | 浅睡 | 摄入新的每日笔记和会话记录，保持索引新鲜 |
| **Deep** | 每天 3am | 深睡 | 从短期记忆中筛选出值得长期保留的内容，晋升到 MEMORY.md |
| **REM** | 每周日 5am | 快眼动 | 模式提取和再巩固——发现跨天重复出现的主题 |

### 短期召回追踪（为 Deep 阶段提供数据）

每次 `memory_search` 调用都会异步记录：
- 这个文本片段被召回了多少次
- 被多少个不同的查询触发
- 跨了多少天
- 涉及哪些主题概念

这些数据写入 `short-term-recall.json`，作为 Deep 阶段晋升打分的输入。

### Deep 阶段晋升打分

从追踪数据中计算加权分数：

| 维度 | 权重 | 思想 |
|------|------|------|
| Relevance | 0.30 | 搜索相关性越高，说明这个信息越有用 |
| Frequency | 0.24 | 被频繁召回 = 用户反复需要这个信息 |
| Diversity | 0.15 | 被不同角度的查询命中 = 信息覆盖面广 |
| Recency | 0.15 | 最近还在被召回 = 仍然相关 |
| Consolidation | 0.10 | 跨多天出现 = 不是一时的需求 |
| Conceptual | 0.06 | 涉及多种主题 = 通用性知识 |

晋升门槛：综合得分 ≥ 0.75、至少被召回 3 次、至少来自 2 个不同查询。达标的内容追加到 `MEMORY.md`。

## 五、存储（Memory Flush）

### 什么时候存

不是每轮对话都存，而是在**对话快要被压缩（compaction）前**，将值得保留的内容持久化到磁盘。判断条件：

- **Token 预估**：当上下文窗口即将用满时（扣除预留空间和提前量后），触发一次 flush
- **文件大小兜底**：会话日志超过 2MB 时强制触发
- **去重保护**：同一个压缩周期内只 flush 一次

### 怎么存

flush 本身不是简单的"把对话存下来"，而是**让 Agent 自己决定存什么**。系统启动一个静默的 agent turn，用专用的 prompt 引导：

- 只写入当天的 `memory/YYYY-MM-DD.md`（按用户时区）
- 如果文件已存在，只追加，不覆盖
- `MEMORY.md`、`DREAMS.md` 等长期文件视为只读
- 没有值得存的内容时，回复静默标记，不做任何写入

这个设计把"什么是值得记住的"这个判断交给了 LLM，而不是用规则硬编码。

## 六、文件结构

```
MEMORY.md                            — 长期持久记忆（Deep 晋升写入、搜索时常青不衰减）
memory/YYYY-MM-DD.md                 — 每日笔记（flush 写入、按日期衰减）
memory/.dreams/short-term-recall.json — 短期召回追踪数据
memory/.dreams/phase-signals.json     — Dreaming 阶段信号
memory/.dreams/events.jsonl           — 事件日志
```

索引：SQLite 数据库（chunks 分表 + FTS5 全文索引 + sqlite-vec 向量索引 + embedding 缓存）。

## 七、插件化设计

Memory 是一个**独占插槽**——同一时间只能有一个 memory 插件活跃（`memory-core` 是默认实现）。但其他插件可以通过两种方式增量扩展：

- **Corpus supplement**：把自己的语料库注入搜索结果（如 wiki 知识库）
- **Prompt supplement**：在 Memory Recall prompt 段后追加自己的指令

核心插件注册四个能力：
1. **promptBuilder** — 决定 system prompt 里写什么
2. **runtime** — 提供 SearchManager 实例（builtin SQLite 或 qmd 后端，qmd 失败自动降级 builtin）
3. **flushPlanResolver** — 决定 flush 的阈值和 prompt 内容
4. **publicArtifacts** — 暴露记忆文件列表供外部查询

设计意图：核心不知道记忆怎么搜、怎么存——这些全部由插件决定。核心只负责在正确的时机（system prompt 组装、agent turn 前置、compaction 前置）调用插件提供的接口。
