# Deploy Server With TiDB Cloud

这份文档只覆盖一种部署方式：`smem` 服务端使用 `TiDB Cloud` 作为数据库。

## 前置条件

- Go 1.25+
- 一个可访问的 TiDB Cloud 集群和数据库
- 一个 OpenAI-compatible LLM endpoint
- 一个可访问的 embedding provider

## 配置加载规则

服务端支持两种配置来源：

1. `config.yaml`
2. 环境变量

如果两边都配置了同一个字段，以 `config.yaml` 为准。

默认会读取 `apps/server/config.yaml`。如果想用别的路径，可以设置：

```bash
export SMEM_CONFIG_FILE='/absolute/path/to/config.yaml'
```

## 必要配置

部署时至少要把下面三类配置补齐：

1. TiDB Cloud 数据库连接
2. LLM 配置
3. Embedding 配置

### 1. TiDB Cloud 数据库连接

最少需要：

```bash
export DB_DSN='user:password@tcp(gateway01.xxx.prod.aws.tidbcloud.com:4000)/smem?tls=true&parseTime=true'
```

如果你配置了 `DB_TLS_SERVER_NAME`，服务端会自动为 DSN 注入 TiDB TLS 配置，因此 `DB_DSN` 里不需要再手写 `tls=tidb`。

```bash
export DB_TLS_SERVER_NAME='gateway01.ap-southeast-1.prod.aws.tidbcloud.com'
```

建议：

- 单独创建数据库 `smem`
- 给服务端使用独立账号
- 只授予该数据库的最小必要权限
- 确认 TiDB Cloud IP allowlist 已放行部署机器

### 2. LLM 配置

`smart ingest` 和 `recall rewrite` 都依赖 LLM。最少需要：

```bash
export OPENAI_API_KEY='your-api-key'
```

常见可选项：

```bash
export OPENAI_BASE_URL='https://api.openai.com/v1'
export OPENAI_CHAT_MODEL='gpt-4.1-mini'
```

说明：

- `OPENAI_BASE_URL` 默认是 `https://api.openai.com/v1`
- `OPENAI_CHAT_MODEL` 默认是 `gpt-4.1-mini`
- 这里的 `OPENAI_*` 实际表示 OpenAI-compatible 接口，不要求必须是 OpenAI 官方服务

### 3. Embedding 配置

服务端必须能生成 embedding，否则 memory 会卡在 `creating`，`recall` 也无法正常工作。

当前支持两种 embedding provider。

#### 方案 A：使用 Ollama

这是当前代码默认值。如果你不显式设置 `EMBEDDING_PROVIDER`，默认就是：

```bash
export EMBEDDING_PROVIDER='ollama'
export EMBEDDING_BASE_URL='http://localhost:11434'
export EMBEDDING_MODEL='bge-m3'
export EMBEDDING_DIM='1024'
```

适用场景：

- 部署机器本机就运行了 Ollama
- 或者 `EMBEDDING_BASE_URL` 能访问到远端 Ollama 服务

#### 方案 B：使用 OpenAI-compatible embedding

如果你希望 embedding 也走 OpenAI-compatible 接口，设置：

```bash
export EMBEDDING_PROVIDER='openai'
export EMBEDDING_BASE_URL='https://api.openai.com/v1'
export EMBEDDING_API_KEY='your-api-key'
export EMBEDDING_MODEL='text-embedding-3-small'
export EMBEDDING_DIM='1536'
```

说明：

- `EMBEDDING_BASE_URL` 为空时，会回退到 `OPENAI_BASE_URL`
- `EMBEDDING_API_KEY` 为空时，会回退到 `OPENAI_API_KEY`
- `EMBEDDING_MODEL` 为空时，会默认使用 `text-embedding-3-small`
- `EMBEDDING_DIM` 必须与模型真实维度一致

## 推荐配置文件

推荐从模板开始：

```bash
cd apps/server
cp config.yaml.example config.yaml
```

下面给一个 TiDB Cloud + OpenAI-compatible LLM + OpenAI-compatible embedding 的完整示例：

```yaml
server_addr: ":8080"

db_dsn: "user:password@tcp(gateway01.xxx.prod.aws.tidbcloud.com:4000)/smem"
db_tls_server_name: "gateway01.ap-southeast-1.prod.aws.tidbcloud.com"

openai_base_url: "https://api.openai.com/v1"
openai_api_key: "your-api-key"
openai_chat_model: "gpt-4.1-mini"

embedding_provider: "openai"
embedding_base_url: "https://api.openai.com/v1"
embedding_api_key: "your-api-key"
embedding_model: "text-embedding-3-small"
embedding_dim: 1536
```

如果你想继续使用环境变量方式，至少要保证：

```bash
export DB_DSN='user:password@tcp(gateway01.xxx.prod.aws.tidbcloud.com:4000)/smem'
export DB_TLS_SERVER_NAME='gateway01.ap-southeast-1.prod.aws.tidbcloud.com'
export OPENAI_API_KEY='your-api-key'
export EMBEDDING_PROVIDER='openai'
export EMBEDDING_DIM='1536'
```

## 启动

```bash
cd apps/server
go run ./cmd/smem-server
```

启动时会自动：

- 连接 TiDB Cloud
- 执行 `AutoMigrate`
- 初始化 HTTP 路由

## 验证

```bash
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/api/v1/memories \
  -H 'Content-Type: application/json' \
  -d '{"content":"remember vim","mode":"normal"}'
```

预期：

- `GET /healthz` 返回 `{"status":"ok"}`
- normal ingest 可成功写入 memory
- smart ingest 在 LLM 可用时能创建原子 memory
- recall 能返回 active memories

## 故障排查

- 数据库连不上：先检查 DSN、用户名密码、数据库名、IP allowlist
- TLS 报错：确认 `DB_TLS_SERVER_NAME` 正确，且网关域名与你的 TiDB Cloud 连接信息一致
- migration 失败：确认数据库已创建，且账号有建表权限
- smart ingest 失败：先检查 `OPENAI_BASE_URL`、`OPENAI_API_KEY`、`OPENAI_CHAT_MODEL`
- memory 长时间停留在 `creating`：先检查 `EMBEDDING_PROVIDER`、`EMBEDDING_BASE_URL`、`EMBEDDING_MODEL`、`EMBEDDING_DIM`
