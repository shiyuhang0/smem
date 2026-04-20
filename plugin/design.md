# Client Plugin Design

## Purpose

针对 openclaw 设计 memory plugin，提供自动记忆存储和召回能力。

## Injection Points

- `agent_end`
  - 每轮用户输入触发一次，不是每会话一次。
  - 需要注意 `agent_end` 还会有前几轮对话内容，设计时需要注意去重，防止重复存储。
  - 格式化信息，然后调用服务端记忆存储接口，存储记忆。
- `before_prompt_build`
  - 提供两种模式：
  - 默认模式：传入 prompt，请求服务端搜索，召回注入到 prompt 中。
  - `prebuild` 模式：除了第一轮对话，直接注入上一轮对话召回的记忆。该模式仅复用相邻轮次的结果；当用户输入主题明显变化时，应放弃复用并重新搜索。

## Key Decision

1. client 去重策略：注入到 prompt 时，用 `memory` 包括召回的记忆内容。然后再 `agent_end` 存储记忆时，去掉召回的 `memory`，防止重复存储。

## Parameters

1. 服务端地址。
2. `EnablePrebuild`：是否启用 `prebuild` 模式。
3. `RecallNum`：召回记忆数量，默认 5 条。