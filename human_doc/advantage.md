

## openclaw plugin

1. 替换 OpenClaw 内置 memory slot。
2. 提供 memory_search 和 memory_store 等 tool，供模型按需调用。
3. 提供两种 recall 模式
   - 默认是 toolMode=true: LLM 决定合适调用 memory_search 和 memory_store tool 调用的时机和内容。system prompt 中注入 guidance，引导模型合理使用工具。
   - toolMode=false: 基于 hook 的自动模式。插件在 before_prompt_build 中主动调用 recall，并把结果注入 prompt。store 在 agent_end 时自动调用。system prompt 中引导模型尽量不主动调用工具，而是依赖自动 recall/store。
4. 自动模式下去重： 清洗 <memory> 注入块，防止 recall 结果再次被 ingest
5. 降级：recall/store 失败均静默降级，不影响主链路。


核心 inject point:
1. memory slot 机制: 通过 kind: "memory" 接入 OpenClaw 的排他 memory 插件体系。由 plugins.slots.memory = "smem-openclaw" 激活
1. registerMemoryCapability({ promptBuilder }): 注入静态 memory guidance 到 system prompt，指导模型如何使用 memory。
2. registerTool：提供 memory_search 和 memory_store tool
3. api.on("before_prompt_build", ...): 每轮对话 recall 时注入 prompt
4. api.on("agent_end", ...): store 注入点