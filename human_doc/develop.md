# develop 日志

## Format

说明时间+具体内容。具体内容3句话以内概括。 example: 

```
2026-04-17 18:00
- design: 重新设计 ingest。<具体内容>
- code: 实现 ingest。<具体内容>
- refactor: 重构 ingest。ingest 实现有误。本次重构 <具体内容>
```

## 前期

1. 调研业界最佳实践
2. 写 design
3. 基于 design 生成 scaffold，定义边界。

## server 开发

### 2026-04-14 ~ 2026-04-15

项目初始化

- code: 实现服务端核心功能。CRUD API + normal/smart ingest + recall pipeline（rewrite/RRF/rerank/softmax）。
- code: 完成基础设施层。GORM TiDB 持久化 + TLS 直连 + YAML 配置文件 + 统一重试策略 + LLM/embedding provider。
- doc: 生成 OpenAPI spec、部署文档、用户文档和 AGENTS.md。

### 2026-04-16

- feature: 支持 embedding 多 provider。
- refactor: 清理无用配置。
- fix: 修复 TiDB 连接与启动链路。DB DSN 统一自动注入；`parseTime=true` 和 `tls=tidb`，避免时间字段扫描/TLS 配置错误；服务启动不再使用 `AutoMigrate`，改为按顺序执行 `apps/server/migrations/*.sql`。

### 2026-04-17 22:00

- refactor: 发现数据库和搜索实现错误。优化数据库，喂 tidb cloud 文档，正确实现向量搜素和全文搜索。并补充日志。
- design: recall 代码繁琐，实现有误，优化 recall design。
- refactor: 基于新 design 重构 recall 。拆成向量搜索、全文搜索、RRF 融合、rerank、softmax、按概率选 topk 等独立方法。
- refacor: 发现无关记忆因时间或热度被错误抬分。优化 rerank 打分策略，引入 `Relevance-Gated Rerank`，先用 `distance`/`score` 计算 `relevance`，仅在相关性超过阈值后才叠加 `recency` 和 `store_count` boost。

### 2026-04-17 24:00

- refactor: 基于 recall 代码风格优化全部代码。

### 2026-04-18 17:00

design: 重新设计 ingest。

### 
