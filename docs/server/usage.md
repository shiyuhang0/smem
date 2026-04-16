# SMEM Server Usage

## 启动后建议验证顺序

1. 调 `GET /healthz`
2. 写入一条 normal memory
3. 查询该 memory
4. 再执行 recall

## 为什么 memory 可能停留在 `creating`

出现这种情况通常是因为 embedding 还未完成，或者 embedding 请求连续失败。当前实现中，只有 embedding 成功后 memory 才会转为 `active`。

## Smart 模式建议

- 输入尽量包含稳定、持久的信息
- 避免把整轮对话摘要直接作为单条 memory 存储
- 当 LLM 抽取失败时，服务端会自动回退为 normal

## Recall 建议

- 短 query 也可直接 recall
- 若主题变化明显，建议直接传入新的原始输入内容
- `top_k` 建议保持在 `1-10`
