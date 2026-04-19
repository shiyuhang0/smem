# Getting Started

这份文档描述如何在本地把 `smem` 的服务端跑起来。

## 1. 准备依赖

- Go 1.25+
- 一个可访问的 TiDB 实例
- 该 TiDB 实例需要支持原生 `VECTOR` 和 `FULLTEXT` 检索能力
- 一个 OpenAI-compatible API endpoint
- 一个可访问的 embedding provider

## 2. 配置文件或环境变量

推荐先使用模板文件：

```bash
cd server
cp config.yaml.example config.yaml
```

然后编辑 `config.yaml`。配置文件的优先级高于环境变量。

如果你更喜欢环境变量方式，也可以继续使用：

最少需要：

```bash
export DB_DSN='user:password@tcp(host:4000)/smem'
export DB_TLS_SERVER_NAME='gateway01.ap-southeast-1.prod.aws.tidbcloud.com'
export OPENAI_API_KEY='your-api-key'
export EMBEDDING_PROVIDER='openai'
export EMBEDDING_DIM='1536'
```

可选：

```bash
export OPENAI_BASE_URL='https://api.openai.com/v1'
export OPENAI_CHAT_MODEL='gpt-4.1-mini'
export EMBEDDING_BASE_URL='https://api.openai.com/v1'
export EMBEDDING_API_KEY='your-api-key'
export EMBEDDING_MODEL='text-embedding-3-small'
export SERVER_ADDR=':8080'
```

如果你不显式设置 embedding 配置，当前默认会使用本机 `Ollama`：

```bash
export EMBEDDING_PROVIDER='ollama'
export EMBEDDING_BASE_URL='http://localhost:11434'
export EMBEDDING_MODEL='bge-m3'
export EMBEDDING_DIM='1024'
```

如果你不想把配置文件放在 `server/config.yaml`，也可以显式指定：

```bash
export SMEM_CONFIG_FILE='/absolute/path/to/config.yaml'
```

## 3. 启动服务端

```bash
cd server
go run ./cmd/smem-server
```

启动时会自动尝试：

- 连接数据库
- 按文件名顺序执行 `server/migrations/*.sql`
- 初始化 HTTP 路由

## 4. 验证服务

```bash
curl http://localhost:8080/healthz
```

## 5. 写入一条测试 memory

```bash
curl -X POST http://localhost:8080/api/v1/memories \
  -H 'Content-Type: application/json' \
  -d '{"content":"remember that I use neovim","mode":"normal"}'
```

## 6. 召回测试

```bash
curl -X POST http://localhost:8080/api/v1/memories/recall \
  -H 'Content-Type: application/json' \
  -d '{"content":"what editor do i use","top_k":5,"temperature":1}'
```

## 7. 跑测试

```bash
cd server
go test ./...
```

如果你要验证真实 TiDB Cloud 连接：

```bash
cd server
SMEM_INTEGRATION_TIDB=1 go test ./internal/tidb -run TestTiDBCloudConnection -v
```
