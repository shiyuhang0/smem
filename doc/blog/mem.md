# Mem 方案调研

本文调研当前 mem 方案的设计和实现。各产品本身在快速发展，调研内容仅基于 2026-04-26 现状。

## 整体比较

基本都是 client + server 的产品形态

| 方案 | server | 存储 | 仓库地址 | stars | 备注 |
|------|----------|----------------|------------------|------------|------|
| mem0 | Python | pg | https://github.com/mem0ai/mem0 | 54k | 业界知名 |
| mem9 | Go | TiDB Cloud zero / db9 / pg | https://github.com/mem9-ai/mem9 | 1k | PingCAP 前端做的很好 |
| clawmem | Go | TiDB Cloud | https://github.com/clawmem-ai/clawmem-openclaw-plugin <br> https://github.com/pingcap/agent-git-service | 3 | PingCAP，后端类 github 仓库，记忆按 github 形式管理，很有意思 |
| supermem | / | / | https://github.com/supermemoryai/supermemory | 22k | server 未开源 |
| powermem | Python | SQLite / PGVector / OceanBase | https://github.com/oceanbase/powermem | 600  | 阿里云。RRF 权重算法 |
| openclaw memory core | TypeScript | Markdown + SQLite | https://github.com/openclaw/openclaw | 364k | openclaw 内置 |
| smem（本项目） | Go | TiDB Cloud | https://github.com/shiyuhang0/smem |  | 自部署；支持 dashboard |

## Ingest

基本都是记忆提取+记忆融合这一套。有些会给记忆做分类，这个挺重要的，后面对记忆的探索会依赖记忆分类。

| 方案 | 记忆提取 | 记忆融合 | 最终写入形态 | 备注 |
|------|----------|----------|--------------|------|
| mem0 | LLM 提取 user + assistant 消息 | LLM 决策 `ADD / UPDATE / DELETE / NONE` | 结构化 memory 表 | 记忆分类：semantic, episodic, procedural |
| mem9 | LLM 提取消息，记忆打 tags | LLM 决策 `ADD / UPDATE / DELETE / NOOP`；`age` 可参与冲突裁决 | 结构化 memory 表，区分 `pinned / insight / digest` 等类型 |  |
| clawmem | LLM 提取消息/摘要，打 kind/label | LLM 决策 save/stale | 结构化存储 | 展示为 git backend 形式，记忆在 GitHub issue |
| powermem | LLM 抽取；`UserMemory` 还会额外抽用户画像 | LLM 决策 `ADD / UPDATE / DELETE / NONE`，并执行对应动作 | 写入统一落到存储适配层；画像会额外写到 `user_profiles` |  |
| openclaw memory core | 让 agent 判断哪些内容值得写入当天 memory 文件 | flush 阶段选择性写入，Deep 阶段晋升到 `MEMORY.md` | `memory/YYYY-MM-DD.md` 追加写入；长期知识晋升到 `MEMORY.md` |  |
| smem（本项目） | 异步 ingest；`smart` 模式 LLM 抽取；支持 `type + kind` 分类 | LLM 执行创建、更新、删除、强化 | 结构化存储 | 异步 job，不阻塞主 agent 路径 |

## Recall

基本是混合检索+ RRF 融合 + 时间衰退机制。

| 方案 | 召回方式 | 精排 | 特点 |
|------|----------|------|------|
| mem0 | metadata filter + vector store + graph store | 可选 rerank | 多路结果直接返回 |
| mem9 | 向量搜索 + 全文检索 + RRF | 按 `pinned=1.5x`、`insight=1.0x` 加权；给结果附加 `relative_age`；尚未加入 rerank | 比较直接 |
| clawmem | 向量检索 + 全文检索 + RRF | 本地 `scoreMemoryMatch` 多粒度打分函数，但还未启用 |  |
| powermem | SQLite/PGVector 是纯向量，OceanBase 支持 dense + full-text + sparse 三路检索 | RRF、可选 rerank | RRF 权重归一化：每路给权重，少了一路就重新分配权重使之为1 |
| openclaw memory core | 向量搜索 + FTS5 BM25；支持 session transcript 和插件补充语料 | weighted merge、可选 MMR、时间衰减、citation 装饰 | 可以及本地文件操作 |
| smem（本项目） | vector search + full-text search + 可选 RRF（头部提取） | `bge-rerank` + 多维度打分；可选 softmax + temperature 发散机制 | RRF 先进行头部提取，倾向于不要 RRF；不认同时间衰退机制 |

## Openclaw memory Plugin

基本都提供了 tool，通过 openclaw memory plugin 替换 openclaw memory slot

| 方案 | 写入时机 | 召回时机 |
|------|----------|----------|
| mem0 | autoCapture mode：`agent_end` <br> skills mode：由 agent 决定 | before_prompt_build |
| mem9 | `agent_end`、`before_reset`；也支持显式 tool 写入 | `before_prompt_build` |
| clawmem | `0.1.16`：`session_end`、`reset`、`agent_end` <br> `0.1.17`：`handleFinalize` | `before_prompt_build`、`before_agent_start` |
| powermem | agent_end；before_agent_start 基于 WAL 前置写入：显式 tool | before_agent_start |
| openclaw memory core | pre-compaction memory flush；后台 Dreaming | `registerMemoryPromptSection` 注入 system prompt recall guidance，运行时调用 `memory_search` / `memory_get` |
| smem（本项目） | `toolMode=true`：显式 `memory_store` <br>`toolMode=false`：`agent_end` 自动写入 | `toolMode=true`：显式 `memory_search` <br> `toolMode=false`: `before_prompt_build` 自动召回 |

- agent_end 是每轮对话结束时，非整个结束的时候
