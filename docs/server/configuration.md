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
- 有效的 embedding 配置：服务端必须能解析出可用的 `EMBEDDING_PROVIDER`、`EMBEDDING_MODEL` 和 `EMBEDDING_DIM`

## Configuration File Keys

配置文件中使用以下 YAML key：

- `server_addr`
- `db_dsn`
- `db_tls_server_name`
- `openai_base_url`
- `openai_api_key`
- `openai_chat_model`
- `embedding_provider`
- `embedding_base_url`
- `embedding_api_key`
- `embedding_model`
- `embedding_dim`

## Optional

- `SERVER_ADDR`: 默认 `:8080`
- `SMEM_CONFIG_FILE`: 显式指定配置文件路径；设置后若文件不存在会报错
- `DB_TLS_SERVER_NAME`: 配置后会自动为 TiDB 连接注册 TLS `ServerName`，并在运行时注入 `tls=tidb`
- `OPENAI_BASE_URL`: 默认 `https://api.openai.com/v1`
- `OPENAI_CHAT_MODEL`: 默认 `gpt-4.1-mini`
- `EMBEDDING_PROVIDER`: 默认 `ollama`
- `EMBEDDING_BASE_URL`: `EMBEDDING_PROVIDER=openai` 时默认回退到 `OPENAI_BASE_URL`；`EMBEDDING_PROVIDER=ollama` 时默认 `http://localhost:11434`
- `EMBEDDING_API_KEY`: `EMBEDDING_PROVIDER=openai` 时默认回退到 `OPENAI_API_KEY`
- `EMBEDDING_MODEL`: `EMBEDDING_PROVIDER=openai` 时默认 `text-embedding-3-small`；`EMBEDDING_PROVIDER=ollama` 时默认 `bge-m3`
- `EMBEDDING_DIM`: `EMBEDDING_PROVIDER=openai` 时默认 `1536`；`EMBEDDING_PROVIDER=ollama` 时默认 `1024`

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

## 配置建议

- 如果部署在远端服务器，除非你明确运行了 Ollama，否则不要依赖默认的 `ollama` 配置
- 使用 `EMBEDDING_PROVIDER=openai` 时，通常还需要同时设置 `EMBEDDING_DIM=1536`
- 使用 `EMBEDDING_PROVIDER=ollama` 时，通常还需要确认 `EMBEDDING_BASE_URL` 可达，且 `EMBEDDING_DIM` 与模型维度一致
- 虽然 embedding 有默认值，但默认会走本机 `Ollama + bge-m3 + 1024`，部署前应明确确认这是否符合你的环境
