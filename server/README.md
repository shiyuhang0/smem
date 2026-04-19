# smem-server

Agent 记忆系统的服务端，提供记忆的存储、管理、智能提取、融合与召回能力。

## 架构

```
cmd/smem-server/          程序入口，启动 HTTP 服务 + 后台 Worker
internal/
├── app/                  依赖注入、路由注册、优雅关闭
├── config/               环境变量 + YAML 配置加载，日志工具
├── handler/              Gin HTTP Handler，DTO 定义，错误映射
├── store/                GORM 模型、Repository 实现、数据库连接、SQL 迁移
├── ai/
│   ├── llm/              LLM 抽象（OpenAI 兼容接口）
│   ├── embedding/        Embedding 抽象（OpenAI / Ollama）
│   └── retry/            共享重试策略（指数退避 + 抖动）
└── domain/
    ├── memory/           核心领域类型、枚举、Repository 接口、Service
    ├── ingestjob/        异步 Ingest Job 实体与生命周期
    ├── ingest/           Normal / Smart 模式存储编排，后台 Job Worker
    └── recall/
        ├── service.go    召回流程：Embed → 双路搜索 → RRF → 精排
        └── scoring/      RRF 融合、复合打分、Softmax 概率选择
```

分层原则：HTTP 关注点不进入 domain 层，domain 层不依赖具体存储实现。

## 快速开始

### 前置依赖

- Go 1.25+
- TiDB Cloud 集群（需要 VECTOR 和 FULLTEXT 支持）
- OpenAI 兼容的 LLM API（如 OpenAI、DeepSeek）
- Embedding API（OpenAI 或 Ollama）

### 配置

复制配置模板并填写：

```bash
cp config.yaml.example config.yaml
```

`config.yaml` 示例：

```yaml
server_addr: ":8080"

db_dsn: "user:password@tcp(gateway01.ap-southeast-1.prod.aws.tidbcloud.com:4000)/smem"
db_tls_server_name: "gateway01.ap-southeast-1.prod.aws.tidbcloud.com"

openai_base_url: "https://api.openai.com/v1"
openai_api_key: "your-api-key"
openai_chat_model: "gpt-5.4"

embedding_provider: "openai"          # openai 或 ollama
embedding_base_url: "https://api.openai.com/v1"
embedding_api_key: "your-api-key"
embedding_model: "text-embedding-3-small"
embedding_dim: 1536
```

也可以通过环境变量配置，环境变量名与 YAML 字段名对应（全大写，如 `DB_DSN`、`OPENAI_API_KEY`）。YAML 值会覆盖环境变量。

### 构建与运行

```bash
cd server
go build ./cmd/smem-server
./smem-server
```

或直接运行：

```bash
go run ./cmd/smem-server
```

服务默认监听 `:8080`，健康检查：`GET /healthz`。

### 测试

```bash
# 单元测试（使用 SQLite 内存数据库）
go test ./...

# 运行特定包
go test ./internal/domain/ingest -v
go test ./internal/domain/recall -v
go test ./internal/handler -v

# TiDB Cloud 集成测试
SMEM_INTEGRATION_TIDB=1 DB_DSN='...' DB_TLS_SERVER_NAME='...' \
  go test ./internal/store -run TestTiDBCloudConnection -v
```

## 主要功能

### Memory CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/memories` | 创建记忆（提交 Ingest Job，返回 202） |
| GET | `/api/v1/memories` | 列表/搜索记忆，支持分页、state/type 过滤 |
| GET | `/api/v1/memories/:id` | 查询单条记忆 |
| PUT | `/api/v1/memories/:id` | 更新记忆内容或状态 |
| DELETE | `/api/v1/memories/:id` | 删除记忆 |
| POST | `/api/v1/memories/recall` | 基于内容召回相关记忆 |

Memory 核心字段：`id`、`content`、`embedding`、`content_hash`、`type`、`kinds`、`scope`、`state`、`metadata`、`version`、`store_count`、`use_count`、`last_accessed_at` 等。

### Memory Type（垂直分类）

- `fact`：事实
- `episodic`：经历/事件
- `procedural`：流程/经验/约定

### Memory Kinds（横向关联）

一条记忆可属于多个 kinds：`skill`、`task`、`lesson`、`workflow`、`preference`、`profile`、`note`、`decision`。

### Memory State

- `creating`：创建中，不参与搜索和召回
- `active`：可用，正常搜索和召回
- `archived`：归档，保留但不参与搜索和召回

### Ingest（记忆存储）

所有创建请求都通过异步 Ingest Job 处理，API 立即返回 202。后台 Worker 轮询执行。

#### Normal Mode

直接存储：用户提供 `content`（可选 `type`、`kinds`、`scope`），系统计算 embedding 和 `content_hash` 后写入。`content_hash` 冲突时自动转更新（`store_count++`）。

#### Smart Mode

智能存储，流程：

1. **信息提取**：基于 LLM 从输入 `content` 中提取最多 5 条原子化候选记忆（`content` + `type` + `kinds`）
2. **相关召回**：对每条候选记忆召回最多 3 条已有记忆，共最多 15 条
3. **记忆融合**：基于 LLM 对候选记忆与已有记忆做 reconcile，输出最终动作：
   - 候选记忆 → `ignore`（无用或已被吸收）或 `create`（创建新记忆）
   - 已有记忆 → `update`（更新内容，即使内容不变也视为更新）、`delete`（冲突删除）或 `ignore`（不变）
4. **事务写入**：所有数据库操作在单一事务中完成

失败重试最多 5 次，线性退避（`now + execute_count` 秒后重试）。

### Recall（记忆召回）

基于输入内容召回最相关的 top-K 条记忆，流程：

1. **查询重写**：预处理阶段（预留扩展）
2. **双路搜索**：
   - **向量搜索**：基于 embedding 的 cosine 距离，召回 2K 条
   - **全文搜索**：基于 TiDB FULLTEXT 索引，召回 2K 条
3. **RRF 融合**：Reciprocal Rank Fusion（k=60）合并两路结果，取 top 2K
4. **Relevance-Gated Rerank**：
   - `relevance = 0.6 × vector_similarity + 0.4 × fulltext_score`
   - 低于阈值（0.2）时不叠加业务信号，防止弱相关记忆被时间和存储次数抬分
   - 高于阈值时：`boost = 0.7 × recency + 0.3 × store_count_score`，最终分 = `relevance + 0.1 × boost`
5. **Top-K 选择**：按得分取前 K 条（Softmax 概率选择已实现，默认关闭）

### Storage（存储）

- **TiDB Cloud**：使用原生 `VECTOR` 列存储 embedding，`vec_cosine_distance()` 做向量搜索，`FULLTEXT ... WITH PARSER MULTILINGUAL` 索引做全文搜索
- **TLS 支持**：通过 `db_tls_server_name` 配置自动启用 TLS 连接
- **自动迁移**：启动时按顺序执行 `migrations/` 下的 SQL 文件
- **乐观并发**：Ingest Job 使用 `worker_id` + `execute_count` + `locked_at` 实现安全的并发 Claim

### LLM 支持

通过 OpenAI 兼容的 `/chat/completions` 接口，支持任意兼容模型（OpenAI、DeepSeek 等）。用于 Smart Ingest 的信息提取和记忆融合。

配置项：`openai_base_url`、`openai_api_key`、`openai_chat_model`。

### Embedding 支持

两种 Provider：

| Provider | 默认模型 | 默认维度 | 说明 |
|----------|----------|----------|------|
| OpenAI | `text-embedding-3-small` | 1536 | 调用 `/embeddings` 接口 |
| Ollama | `bge-m3` | 1024 | 调用 `/api/embed` 接口 |

通过 `embedding_provider` 切换。所有 embedding 调用使用共享重试策略（3 次重试，指数退避 + 抖动）。

## 创新点

### 1. Smart Ingest：提取-召回-融合三阶段

不是简单地把用户输入直接存入数据库，而是：
- 用 LLM 从原始输入中提取原子化记忆（最多 5 条）
- 对每条候选记忆召回已有相关记忆
- 用 LLM 做结构化的融合决策（create/update/delete/ignore），保证记忆不重复、不冲突

这使得记忆库始终保持原子化、去重、一致的长期状态。

### 2. Relevance-Gated Rerank

召回打分中，业务信号（时间新鲜度、存储次数）不能脱离检索相关性独立抬分。通过 relevance gate 机制，只有相关性超过阈值的候选才能获得业务信号加成，避免"新但无关"的记忆污染召回结果。

### 3. 双路搜索 + RRF 融合

同时利用向量语义搜索和全文关键词搜索，通过 Reciprocal Rank Fusion 合并排序，兼顾语义相似性和精确关键词匹配。相比单路搜索有更好的召回覆盖。

### 4. 异步 Ingest + 乐观并发

所有写入通过 Ingest Job 异步执行，API 立即返回 202。Worker 使用乐观锁（`execute_count` + `worker_id`）安全并发处理，支持失败重试和重启恢复。

### 5. Content Hash 去重

通过 `content_hash`（SHA-256）精确去重。相同内容的记忆不会重复创建，而是累加 `store_count`，自然地衡量记忆的重要程度，参与后续精排。
