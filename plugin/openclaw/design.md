# Client Plugin Design

## Purpose

针对 openclaw 设计 memory plugin，提供自动记忆存储和召回能力。

## Key Decision

1. `smem-openclaw` 占用 `plugins.slots.memory`，替换当前 active memory plugin。
2. memory 主路径通过 `toolMode` 切换。
   1. `toolMode=true` 时采用 tool-driven 方案：通过 `memory_search` / `memory_store` 对接 `smem server`，并在 prompt 中说明模型按需使用。
   2. `toolMode=false` 时采用 hook-based 自动模式：client 用 `<memory>` 包裹召回内容，并在 `agent_end` 存储前移除，防止重复存储。
3. 两个工具始终注册，但在自动模式下 prompt 不再主动引导模型调用它们。

## Parameters

1. 服务端地址: `ServerURL`，默认 `http://localhost:5173`。
2. `toolMode`：是否使用 tool-driven 模式，默认 `true`。

## Recall

tool 驱动型 recall
   - `toolMode=true` 时，插件通过 `memory_search` tool 对接 `POST /api/v1/memories/recall`。
   - 通过 `registerMemoryCapability({ promptBuilder })` 注入静态 guidance，引导模型按需使用 `memory_search`。

Hook based recall
   - `toolMode=false` 时，在 `before_prompt_build` 中读取当前 prompt / messages。
   - 插件主动请求 `POST /api/v1/memories/recall`。
   - 将结果格式化后直接注入 prompt，使用 `<memory>` 包裹。后续 store 时再去掉该块，避免重复存储。

## Store

自动 store
   - `toolMode=false` 时，在 `agent_end` 中整理本轮 user 内容。
   - 调用 `POST /api/v1/memories`。

显式 store
   - 插件注册 `memory_store` tool。
   - `toolMode=true` 时作为默认 store 路径，对接 `POST /api/v1/memories`。

## SMEM Replacement Scope

本节明确 `smem-openclaw` 在替换 OpenClaw `memory` slot 后，第一版实际实现的范围。

### What We Will Implement

第一版实现聚焦于“替换 memory 主链路”，支持 tool-driven 和 hook-based automatic 两种模式，不追求兼容 OpenClaw 内置 Markdown memory 生态。

1. 实现真正的 `memory` plugin。
   - 在 `openclaw.plugin.json` 中声明 `kind: "memory"`。
   - 通过 `plugins.slots.memory = "smem-openclaw"` 占用 memory slot。
2. 实现 `memory_search` tool。`toolMode=true` 时作为默认 recall 路径。
3. 实现 `agent_end` store。`toolMode=false` 时作为默认 store 路径。
   - 对本轮内容做整理，调用 `POST /api/v1/memories`。
   - 写入 `agent_id`、`session_id`、`source`、`metadata` 等上下文字段。
4. 实现 `memory_store` tool。工具始终注册，但只在 `toolMode=true` 时作为默认 store 路径。
5. 实现注入内容去重。
   - 仅在 `toolMode=false` 时，recall 注入统一使用 `<memory>` 包裹。
   - `agent_end` 存储前去掉该块，避免 recall 内容再次被 ingest。
5. 实现失败降级。
   - recall 失败时静默跳过，不影响主链路。
   - store 失败时只记日志，不影响主链路。

### What We Will Not Implement In V1

第一版明确不实现以下能力：

1. OpenClaw 默认 Markdown memory 文件写入与读取。
2. 本地 memory search backend、embedding provider、本地索引管理。
3. `memory_get` 等补充型 memory tool。
4. compaction memory flush、public artifacts、memory capability、memory runtime 深度扩展能力。

### Example Configuration

```json
{
  "plugins": {
    "enabled": true,
    "slots": {
      "memory": "smem-openclaw"
    },
    "entries": {
        "smem-openclaw": {
          "enabled": true,
          "config": {
            "serverURL": "http://localhost:5173",
            "toolMode": true,
            "topK": 5,
            "storeMode": "smart",
            "timeoutMs": 8000
          }
        }
      }
  }
}
```
