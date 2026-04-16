# SMEM Server Configuration

服务端现在同时支持配置文件和环境变量。

优先级规则：

1. 配置文件
2. 环境变量
3. 代码默认值

默认会尝试读取 `apps/server` 工作目录下的 `config.yaml`。也可以通过 `SMEM_CONFIG_FILE` 显式指定配置文件路径。

模板文件位于 `apps/server/config.yaml.example`。

## Required

- `DB_DSN`: TiDB 连接串
- `OPENAI_API_KEY`: OpenAI-compatible API key

## Configuration File Keys

配置文件中使用以下 YAML key：

- `server_addr`
- `db_dsn`
- `db_tls_server_name`
- `openai_base_url`
- `openai_api_key`
- `openai_chat_model`
- `openai_embedding_model`
- `embedding_dim`
- `recall_default_topk`
- `recall_max_topk`
- `recall_temperature`
- `enable_fulltext`
- `log_level`

## Optional

- `SERVER_ADDR`: 默认 `:8080`
- `SMEM_CONFIG_FILE`: 显式指定配置文件路径；设置后若文件不存在会报错
- `DB_TLS_SERVER_NAME`: 当 `DB_DSN` 使用 `tls=tidb` 时可选，用于注册 TiDB TLS `ServerName`
- `OPENAI_BASE_URL`: 默认 `https://api.openai.com/v1`
- `OPENAI_CHAT_MODEL`: 默认 `gpt-4.1-mini`
- `OPENAI_EMBEDDING_MODEL`: 默认 `text-embedding-3-small`
- `EMBEDDING_DIM`: 默认 `1536`
- `RECALL_DEFAULT_TOPK`: 默认 `5`
- `RECALL_MAX_TOPK`: 默认 `10`
- `RECALL_TEMPERATURE`: 默认 `1.0`
- `ENABLE_FULLTEXT`: 默认 `true`
- `LOG_LEVEL`: 默认 `info`

## 重试策略

所有 LLM 和 embedding 请求都使用统一重试策略：

- 最多 3 次
- 对 `429`、`5xx`、网络错误、超时做重试
- 对普通 `4xx` 不重试
- 使用指数退避加轻量 jitter

## 降级行为

- smart ingest 的 LLM 抽取失败时，自动降级为 normal ingest
- recall 的 query rewrite 失败时，自动回退为原始 query
- fulltext 搜索失败时，只返回 vector 路径结果
