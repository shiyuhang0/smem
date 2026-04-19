# SMEM Server Overview

`server/` 是本项目的 memory 服务端实现，提供 memory 的存储、管理、搜索和召回能力。

## 支持能力

- `POST /api/v1/memories`: 创建 memory，支持 `normal` 和 `smart` 模式
- `GET /api/v1/memories/{id}`: 查询单条 memory
- `PUT /api/v1/memories/{id}`: 显式更新 memory
- `DELETE /api/v1/memories/{id}`: 删除 memory
- `GET /api/v1/memories`: 分页列表和关键字搜索
- `POST /api/v1/memories/recall`: 基于输入内容召回相关记忆

## Memory Model

- `type`: `fact | episodic | procedural`
- `scope`: `user | agent | external`
- `state`: `creating | active | archived`
- `kinds`: 横向分类数组，如 `preference`、`note`、`workflow`

当前实现中：

- `creating` 不参与默认 recall
- `active` 参与 recall
- `archived` 保留但不参与 recall

## Ingest 模式

### normal

直接写入一条 memory，先落库为 `creating`，随后执行 embedding，成功后切换为 `active`。

### smart

先调用 LLM 抽取原子化记忆，再对每条候选执行融合决策：

- `ignore`: 丢弃无长期价值内容
- `create`: 创建新 memory
- `update`: 更新已有 memory

若 LLM 抽取失败，会自动降级为 normal 模式。

## Recall 流程

当前实现的 recall 流程为：

1. 尝试用 LLM 重写 query
2. 尝试从 rewrite 结果中提取 `content/type/kinds`
3. 从 active memories 中做向量/文本粗排
4. 用关键字搜索补充候选
5. 通过 RRF 融合两个候选集合
6. 使用 `type/kinds/store_count/updated_at` 做规则精排
7. 对分数做 softmax，返回 top k

若 query rewrite 失败，会退回原始 query。
