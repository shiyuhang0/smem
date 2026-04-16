# SMEM

`smem` 是一个面向 Agent 的个人记忆系统项目，当前仓库包含：

- `apps/server`: Go 服务端，负责 memory 存储、管理、搜索和召回
- `apps/plugin-openclaw`: 预留的 openclaw memory plugin 目录
- `apps/dashboard`: 预留的 dashboard 目录

## 当前状态

服务端已经具备一个可运行的 MVP：

- memory CRUD API
- `normal` / `smart` 两种 ingest 模式
- TiDB 存储
- OpenAI-compatible LLM 与 embedding provider
- LLM/embedding 统一 3 次重试
- recall 的 rewrite、hybrid merge、rerank

## 快速入口

- 服务端概览：`docs/server/overview.md`
- 服务端 API：`docs/server/api.md`
- 服务端配置：`docs/server/configuration.md`
- TiDB Cloud 部署：`docs/deploy/server-tidb-cloud.md`
- 仓库快速上手：`docs/getting-started.md`

## 目录概览

```text
apps/
  server/
  plugin-openclaw/
  dashboard/
docs/
  deploy/
  server/
packages/
```

## 当前优先级

当前主要实现集中在 `apps/server`。plugin 和 dashboard 目录已预留，但尚未进入功能开发阶段。
