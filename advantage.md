# Advantage of SMEM


## 架构

服务端和 plugin 解耦

## recall

hybrid recall：

粗排：vector search + fulltext search + RRF
精排 rerank: 设计了基于 relevance-gated 的多维度打分策略：
- vector search + fulltext search 作为 relevance
- 当 relevance 过阈值（0.2），再加 0.1*boost (更新时间，存储次数，类型等)

好处：类似的记忆，多维度去召回
- 更新的：新记忆优先召回
- 记得更多的会被优先召回。比如：我爱吃饭，我爱吃面（多次记忆），当问最爱吃什么，爱吃面优先召回
- 类型，和问题匹配的类型优先召回
- 其他：

## Ingest

- smart ingest：智能召回
- normal ingest: 直接向量化后存储

smart ingest 设计关键点（prompt 工程）
1. 异步
2. 自动提取关键信息：
3. 记忆融合：定义  ignore | create | update | conflict。不同类型的数据还可以有不同的融合策略。
4. 其他：去重 content hash 

## plugin

client plugin memory 包裹去重

## 以后可以做的

1. 不同类型定义不同融合策略：
- append-only for episodic
- overwrite for canonical preference/profile
- close old + create new for conflicting facts
- require confidence threshold for auto-update

## Future

1. recall 第一步是对 query 先走 LLM 抽取 content/type/kind


3. 失败降级策略
长期记忆系统不要让 LLM/embedding 故障拖垮主链路。
建议明确：
- embedding 失败时：是否先存 raw memory 待补 embedding
- LLM 提取失败时：是否退回 raw ingest
- recall 服务失败时：plugin 是否静默跳过
- full-text 不可用时：是否只跑 vector

## Other Ref

- ignore：无长期价值
- create：新 memory
- update：更新现有 canonical memory
- merge：和多个 memory 合并
- archive：旧 memory 失效
- conflict：发现互相矛盾但不能自动决定