# Deploy Server Locally

## Prerequisites

- Go 1.25+
- 一个可访问的 TiDB 实例
- 一个 OpenAI-compatible API endpoint

## Required Environment Variables

如果你更希望使用配置文件，先从模板生成本地配置：

```bash
cd apps/server
cp config.yaml.example config.yaml
```

`config.yaml` 中的值优先于环境变量。配置文件未填写的字段，才会回退到环境变量或默认值。

如果不用配置文件，也可以继续只使用环境变量：

```bash
export DB_DSN='user:password@tcp(gateway01.xxx.prod.aws.tidbcloud.com:4000)/smem?tls=true&parseTime=true'
export OPENAI_API_KEY='your-api-key'
```

可选：

```bash
export SERVER_ADDR=':8080'
export DB_TLS_SERVER_NAME='gateway01.xxx.prod.aws.tidbcloud.com'
export OPENAI_BASE_URL='https://api.openai.com/v1'
export OPENAI_CHAT_MODEL='gpt-4.1-mini'
export OPENAI_EMBEDDING_MODEL='text-embedding-3-small'
```

如果你的 DSN 使用 `?tls=tidb`，同时设置 `DB_TLS_SERVER_NAME`，服务端会自动注册对应 TLS 配置。

也可以显式指定配置文件路径：

```bash
export SMEM_CONFIG_FILE='/absolute/path/to/config.yaml'
```

## Start

```bash
cd apps/server
go run ./cmd/smem-server
```

## Verify

```bash
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/api/v1/memories \
  -H 'Content-Type: application/json' \
  -d '{"content":"remember vim","mode":"normal"}'
```

## Notes

- 启动时会自动执行 `AutoMigrate`
- 若 OpenAI-compatible endpoint 不可用，smart ingest 和 embedding 会失败
- 当前代码层已实现 3 次重试
