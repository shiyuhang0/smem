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


# Design Details

- [server](../server/design.md)
- [dashboard](../dashboard/design.md)
- [client-plugin](../plugin/design.md)

# Future

以下为未来设想，本次实现不应读取/考虑以下诉求。

## Chunking

以下考虑做 chunking
- 长文档（文章、PDF、网页）— 超过 embedding 模型的 token 上限（通常 512-8192 tokens）
- 异构内容混合 — 一个文档包含多个主题，需要按语义边界拆分以提升检索精度
- 需要细粒度召回 — 用户 query 只和文档某一小段相关，返回整篇文档噪声太大

## Digest

考虑在 openclaw `session_end`、`reset` 时获取最近 k 轮对话内容，提取摘要，并存储。

## 信息提取 in Recall 

召回时可以考虑先做一遍提取。

## 记忆融合优化

已识别到的 bug: conflict 时 store count 增加，目标不增加。

优化融合参考：

- ignore：无长期价值
- create：新 memory
- update：更新现有 canonical memory
- merge：和多个 memory 合并
- archive：旧 memory 失效
- conflict：发现互相矛盾但不能自动决定

不同类型定义不同融合策略：
- append-only for episodic
- overwrite for canonical preference/profile
- close old + create new for conflicting facts
- require confidence threshold for auto-update

## 失败降级 

失败降级策略：长期记忆系统不要让 LLM/embedding 故障拖垮主链路。
- embedding 失败时：是否先存 raw memory 待补 embedding
- LLM 提取失败时：是否退回 raw ingest
- recall 服务失败时：plugin 是否静默跳过
- full-text 不可用时：是否只跑 vector

## Tools & MCP &CLI

提供 MCP/CLI
