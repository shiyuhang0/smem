# SMEM Server API

OpenAPI 文件位于 `apps/server/api/openapi.yaml`。

## Health

```bash
curl http://localhost:8080/healthz
```

## Create Memory

```bash
curl -X POST http://localhost:8080/api/v1/memories \
  -H 'Content-Type: application/json' \
  -d '{
    "content": "User prefers vim",
    "mode": "normal",
    "type": "fact",
    "kinds": ["preference"],
    "scope": "user"
  }'
```

## Create Smart Memory

```bash
curl -X POST http://localhost:8080/api/v1/memories \
  -H 'Content-Type: application/json' \
  -d '{
    "content": "I usually use vim and prefer short commit messages.",
    "mode": "smart"
  }'
```

## Get Memory

```bash
curl http://localhost:8080/api/v1/memories/<memory-id>
```

## Update Memory

```bash
curl -X PUT http://localhost:8080/api/v1/memories/<memory-id> \
  -H 'Content-Type: application/json' \
  -d '{"state": "archived"}'
```

## List Memories

```bash
curl 'http://localhost:8080/api/v1/memories?page=1&page_size=20&search=vim'
```

## Recall Memories

```bash
curl -X POST http://localhost:8080/api/v1/memories/recall \
  -H 'Content-Type: application/json' \
  -d '{"content": "editor preference", "top_k": 5, "temperature": 1}'
```

## Error Shape

```json
{
  "error": "memory not found"
}
```
