# Advantage of SMEM

- 服务端和 plugin 解耦
- 分类 type / kind ，记录更新时间，记录存储次数，这样都可以作为后续记忆管理和召回的维度。
- 支持 smart ingest + hybrid recall
  - smart ingest：基于 LLM 的智能存储，自动提取关键信息，并进行记忆融合。
  - 记忆融合：记忆融合是关键，定义  ignore | create | update | conflict。不同类型的数据还可以有不同的融合策略。
  - hybrid recall: 粗排+精排
- 计划做 dashboard，而不是只做“黑盒向量库”
- 去重：content hash + client memory 包裹去重


- ignore：无长期价值
- create：新 memory
- update：更新现有 canonical memory
- merge：和多个 memory 合并
- archive：旧 memory 失效
- conflict：发现互相矛盾但不能自动决定

## 以后可以做的

不同类型定义不同融合策略：
- append-only for episodic
- overwrite for canonical preference/profile
- close old + create new for conflicting facts
- require confidence threshold for auto-update


recall 流程太依赖 LLM 抽取，成本和稳定性会偏高
- 你现在 recall 第一步是对 query 先走 LLM 抽取 content2/type/kind
- 这个在质量上可能有帮助，但工业上通常不会对每次 recall 都强依赖 LLM preprocessing
- 原因：
  - 增加延迟
  - 增加费用
  - query 轻量时收益不大
  - 容易因 prompt 漂移影响召回稳定性
更常见的做法：
- 默认直接 hybrid search
- 只在复杂 query 或 recall miss 时，再走 query rewrite / extraction
- 或做成可配置策略：
  - raw
  - rewrite_if_needed
  - always_rewrite

8. 失败降级策略
长期记忆系统不要让 LLM/embedding 故障拖垮主链路。
建议明确：
- embedding 失败时：是否先存 raw memory 待补 embedding
- LLM 提取失败时：是否退回 raw ingest
- recall 服务失败时：plugin 是否静默跳过
- full-text 不可用时：是否只跑 vector