# SMEM

`smem` 是一个面向 Agent 的个人记忆系统项目，当前仓库包含：

- `server/`: Go 服务端，负责 memory 存储、管理、搜索和召回
- `dashboard/`: React dashboard，用于搜索、浏览、查看和归档 memories
- 预留：`plugin-openclaw` memory plugin

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
server/          # Go module（module: smem/apps/server）
docs/
  deploy/
  server/
human_doc/
```

## 当前优先级

当前主要实现集中在 `server/` 和 `dashboard/`，plugin 仍处于预留阶段。

## Dashboard

本地运行 dashboard：

```bash
cd dashboard
npm install
npm run dev
```

手动验证清单：

1. 打开 dashboard，确认第一页 memories 正常加载。
2. 输入关键词，确认列表按搜索词刷新。
3. 切换 `kind`，确认卡片列表随筛选变化。
4. 滚动到底部，确认下一页自动加载。
5. 点开一条 memory，确认右侧详情抽屉展示完整内容和 metadata。
6. 在详情抽屉点击归档，确认状态刷新为 `archived`。
7. 暂时停掉服务端，确认列表或详情能显示错误和重试状态。
