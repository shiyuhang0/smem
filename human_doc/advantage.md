# Advantage of SMEM

记录优点/创新点。

## 架构

client + 服务端的架构，client 以 plugin 形式注入 agent。

解耦合
- 服务端专注记忆处理
- client 调用 CRUD。

## recall

1. hybrid 粗排: vector search + fulltext search + RRF
2. 打分精排: （如何解决记忆和业务相关，如最近的记忆优先，多次加深的记忆优先等）设计了 relevance-gated 的多维度打分策略：
   - vector search + fulltext search 作为 relevance。保证主要分数来源是相似性。
   - 当 relevance 过阈值（0.2），再加 0.1*boost (更新时间，存储次数，类型等)，多维度打分。
3. 快速：（解决速度和确认性）recall 只有和数据库的网络交互，不依赖任何大模型。速度有保障，确认性强。

多维度召回的好处
- 更新记忆优先召回。
- 加深的记忆优先召回。比如：我爱吃饭，我爱吃面（多次记忆），当问最爱吃什么，爱吃面优先召回
- 类型，和问题匹配的类型优先召回
- 其他记忆相关特点。

softmax:（如何解决老记忆不被召回了，比如有些老记忆因为时间太旧总是不召回）softmax + temperature，打分后算概率，按概率召回，而不是直接选择 topK。这样记忆会更发散。

## Ingest

1. 两种 ingest 模式： smart ingest + normal ingest （直接向量化后存储）
2. 异步 ingest
3. content hash 去重
4. 智能召回：LLM 提取关键信息 + LLM 记忆融合 （做了 prompt 工程）
   1. 新记忆定义  ignore | create ；老记忆定义 update | delete ｜ ignore。

记忆融合主要情况分析
- 新记忆：创建新的
- 冲突记忆：创建新的，删除老的
- 无用记忆：ignore
- 相似记忆：ignore + 更新老记忆（同时更新 store_count）

