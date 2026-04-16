# Deploy Server With TiDB Cloud

## TiDB Cloud Connection

准备一个 TiDB Cloud 数据库和可连通的 DSN，例如：

```bash
DB_DSN='user:password@tcp(gateway01.xxx.prod.aws.tidbcloud.com:4000)/smem?tls=true&parseTime=true'
```

如果使用自定义 TLS 名称，例如 `?tls=tidb`，再配置：

```bash
DB_TLS_SERVER_NAME='gateway01.ap-southeast-1.prod.aws.tidbcloud.com'
```

建议：

- 单独创建数据库 `smem`
- 给 server 使用独立账号
- 只授予该数据库的最小必要权限

## TLS

TiDB Cloud 场景通常需要开启 TLS。具体参数以你的 TiDB Cloud 连接串为准。

## 启动方式

```bash
cd apps/server
go run ./cmd/smem-server
```

## 验证项

- `GET /healthz` 返回 `{"status":"ok"}`
- 可以成功创建 normal memory
- smart ingest 在 LLM 正常时可创建原子 memory
- recall 返回 active memories

## 故障排查

- DSN 连不上：先确认 IP allowlist 和用户名密码
- migration 失败：确认数据库存在且账号有建表权限
- smart ingest 失败：先检查 `OPENAI_BASE_URL` 和 `OPENAI_API_KEY`
- memory 长时间停留在 `creating`：检查 embedding provider 返回和网络状态
