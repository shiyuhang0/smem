# OpenClaw Memory Core 调研

## 概览

OpenClaw 的 memory 系统通过 **memory slot** 机制实现，一次只有一个 memory 插件活跃。默认使用 `memory-core`（bundled plugin），备选 `memory-lancedb`。Memory slot 由配置 `plugins.slots.memory` 控制。

关键源码位置：
- Memory slot 注册与状态管理：`src/plugins/memory-state.ts`
- Memory slot 加载与门控：`src/plugins/loader.ts`、`src/plugins/config-state.ts`
- System prompt 注入：`src/agents/system-prompt.ts`
- Memory flush 执行：`src/auto-reply/reply/agent-runner-memory.ts`
- Flush plan 构建：`extensions/memory-core/src/flush-plan.ts`
- Memory 工具定义：`extensions/memory-core/src/tools.ts`
- Prompt section builder：`extensions/memory-core/src/prompt-section.ts`
- Search manager：`extensions/memory-core/src/memory/search-manager.ts`
- Memory index manager：`extensions/memory-core/src/memory/manager.ts`

---

## 一、召回（Recall）机制

### 1.1 使用的 Hook

Memory 召回**不使用**任何 plugin lifecycle hook（如 `before_prompt_build`）。它使用专属的 `registerMemoryPromptSection` API（`src/plugins/registry.ts:1037`），由 memory 插件在加载时注册一个 `MemoryPromptSectionBuilder`。

注册链路：
1. memory-core 在 `extensions/memory-core/index.ts:28` 调用 `api.registerMemoryPromptSection(buildPromptSection)`
2. 注册进入 `src/plugins/memory-state.ts:78` 的 `memoryPluginState.promptBuilder`
3. 运行时由 `buildMemoryPromptSection()` 调用（`src/plugins/memory-state.ts:82`）

### 1.2 System Prompt 注入

在 `buildAgentSystemPrompt()`（`src/agents/system-prompt.ts:391-495`）中，调用 `buildMemorySection()`，后者调用 `buildMemoryPromptSection()`，将 memory guidance 以 `## Memory Recall` section 注入到 **system prompt** 中。

**注入的是工具使用指引（不是实际记忆内容）**，指导 LLM 在回答前先调用 `memory_search`/`memory_get`。

#### 具体的 Memory Guidance（`extensions/memory-core/src/prompt-section.ts`）

根据可用工具组合有三种变体：

**同时有 `memory_search` + `memory_get`：**
```
## Memory Recall
Before answering anything about prior work, decisions, dates, people, preferences, or todos: run memory_search on MEMORY.md + memory/*.md; then use memory_get to pull only the needed lines. If low confidence after search, say you checked.
Citations: include Source: <path#line> when it helps the user verify memory snippets.
```

**只有 `memory_search`：**
```
## Memory Recall
Before answering anything about prior work, decisions, dates, people, preferences, or todos: run memory_search on MEMORY.md + memory/*.md and answer from the matching results. If low confidence after search, say you checked.
Citations: include Source: <path#line> when it helps the user verify memory snippets.
```

**只有 `memory_get`：**
```
## Memory Recall
Before answering anything about prior work, decisions, dates, people, preferences, or todos that already point to a specific memory file or note: run memory_get to pull only the needed lines. If low confidence after reading them, say you checked.
Citations: include Source: <path#line> when it helps the user verify memory snippets.
```

当 `citationsMode === "off"` 时，最后一行替换为：
```
Citations are disabled: do not mention file paths or line numbers in replies unless the user explicitly asks.
```

如果两个工具都没有，返回空数组，不注入任何内容。

### 1.3 实际召回过程

LLM 根据 system prompt 中的 guidance，通过 **tool call** 主动检索记忆：

1. **`memory_search`**（`extensions/memory-core/src/tools.ts:24`）：
   - 参数：`query`（必需）、`maxResults`、`minScore`
   - 语义搜索 `MEMORY.md` + `memory/*.md`（及可选的 session transcripts）
   - 使用混合检索：向量相似度 + FTS（全文搜索）
   - 结果经过 MMR 重排序、时间衰减、citation 装饰
   - 返回 JSON：`{ results, provider, model, fallback, citations, mode }`

2. **`memory_get`**（`extensions/memory-core/src/tools.ts:81`）：
   - 参数：`path`（必需）、`from`、`lines`
   - 读取指定 memory 文件的指定行
   - 用于精确提取 search 结果中的片段

召回结果作为 **tool result** 返回到当前对话上下文中（user message 角色）。

### 1.4 检索技术栈

- **Embedding**：通过 `MemoryEmbeddingProviderAdapter` 注册，支持 OpenAI、Mistral 等多种 provider
- **向量存储**：内置使用 `sqlite-vec` 扩展的 SQLite 数据库
- **混合检索**（`extensions/memory-core/src/memory/hybrid.ts`）：
  - 向量相似度搜索（semantic）
  - BM25 全文搜索（keyword/FTS）
  - 两者通过 weighted merge 合并结果
- **MMR 重排序**（`extensions/memory-core/src/memory/mmr.ts`）：减少结果间的冗余
- **时间衰减**（`extensions/memory-core/src/memory/temporal-decay.ts`）：较新的记忆得分更高

---

## 二、存储（Capture/Flush）机制

### 2.1 Memory-core：Pre-compaction Memory Flush

这是 memory-core 的**核心存储路径**。由 `src/auto-reply/reply/agent-runner-memory.ts` 驱动。

#### 触发条件

在 `src/auto-reply/reply/memory-flush.ts:53` 的 `shouldRunMemoryFlush()` 判断：

- **Token 阈值触发**：当前 token 数 ≥ `contextWindow - reserveTokensFloor - softThresholdTokens`
  - `softThresholdTokens` 默认 4000（`extensions/memory-core/src/flush-plan.ts:10`）
  - `reserveTokensFloor` 来自 compaction 配置
- **Transcript 大小强制触发**：transcript 文件大小 ≥ `forceFlushTranscriptBytes`（默认 2MB）
- **去重**：每个 compaction cycle 最多 flush 一次（通过 `memoryFlushCompactionCount` 追踪）
- **排除条件**：heartbeat 轮次、CLI provider、sandbox 只读模式不触发

#### 执行流程

1. **生成 Flush Plan**（`extensions/memory-core/src/flush-plan.ts:95`）：
   - 计算目标文件路径：`memory/YYYY-MM-DD.md`（按用户时区日期）
   - 构造 flush prompt 和 system prompt

2. **启动独立 Agent 运行**（`src/auto-reply/reply/agent-runner-memory.ts:694`）：
   - `trigger: "memory"` 标记为 memory flush 类型
   - `silentExpected: true` 期望静默回复
   - 使用 `runWithModelFallback` 支持模型降级
   - 传入 `memoryFlushWritePath` 供 tool 使用

3. **Flush User Prompt**（默认值，`extensions/memory-core/src/flush-plan.ts:25`）：
   ```
   Pre-compaction memory flush.
   Store durable memories only in memory/YYYY-MM-DD.md (create memory/ if needed).
   Treat workspace bootstrap/reference files such as MEMORY.md, SOUL.md, TOOLS.md, and AGENTS.md as read-only during this flush; never overwrite, replace, or edit them.
   If memory/YYYY-MM-DD.md already exists, APPEND new content only and do not overwrite existing entries.
   Do NOT create timestamped variant files (e.g., YYYY-MM-DD-HHMM.md); always use the canonical YYYY-MM-DD.md filename.
   If nothing to store, reply with NO_REPLY.
   Current time: <current time>
   ```

4. **Flush System Prompt**（默认值，`extensions/memory-core/src/flush-plan.ts:34`）：
   ```
   Pre-compaction memory flush turn.
   The session is near auto-compaction; capture durable memories to disk.
   Store durable memories only in memory/YYYY-MM-DD.md (create memory/ if needed).
   Treat workspace bootstrap/reference files such as MEMORY.md, SOUL.md, TOOLS.md, and AGENTS.md as read-only during this flush; never overwrite, replace, or edit them.
   If memory/YYYY-MM-DD.md already exists, APPEND new content only and do not overwrite existing entries.
   You may reply, but usually NO_REPLY is correct.
   ```

5. **LLM 自主决策**：LLM 分析当前对话上下文，决定哪些内容值得持久化，通过 `write`/`edit` tool 写入 `memory/YYYY-MM-DD.md` 文件

6. **后处理**：
   - 更新 session entry 的 `memoryFlushAt` 和 `memoryFlushCompactionCount`
   - 内容写入后，memory-core 的 search manager 会重新索引这些 `.md` 文件

#### 可配置项

通过 `agents.defaults.compaction.memoryFlush` 配置：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `enabled` | `true` | 是否启用 memory flush |
| `softThresholdTokens` | `4000` | 距 compaction 多少 token 时触发 |
| `forceFlushTranscriptBytes` | `2MB` | transcript 文件大小阈值 |
| `prompt` | 见上方 | 自定义 flush prompt |
| `systemPrompt` | 见上方 | 自定义 flush system prompt |

### 2.2 Memory-lancedb：`agent_end` Hook 自动捕获

`extensions/memory-lancedb/index.ts:566` 注册 `agent_end` hook，实现自动捕获：

1. 对话结束后，提取 **user messages** 中的文本内容（排除 assistant 消息以避免自我污染）
2. 通过 `shouldCapture()` 过滤值得记忆的内容
3. 用 embedding model 生成向量
4. 检查是否与已有记忆重复（余弦相似度 > 0.95 则跳过）
5. 存入 LanceDB：`db.store({ text, vector, importance: 0.7, category })`
6. 每次对话最多捕获 3 条记忆

---

## 三、Memory Index 与同步

### 3.1 索引构建

Memory-core 的 search manager（`extensions/memory-core/src/memory/manager.ts`）负责维护索引：

- **文件监控**：watch `MEMORY.md` 和 `memory/*.md` 文件变化
- **Embedding 批处理**：将文件内容分块后批量生成 embedding
- **向量化存储**：存入 SQLite + sqlite-vec 向量索引
- **FTS 索引**：同时维护 BM25 全文搜索索引

### 3.2 同步触发

- **文件变更**：watcher 检测到 `memory/` 目录文件变化时触发增量同步
- **定时同步**：周期性后台 sync（`manager-sync-ops.ts`）
- **Compaction 后**：post-compaction session memory reindex（`memoryReindex` 配置项，默认 `"async"`）

---

## 四、两种 Memory 插件对比

| 维度 | memory-core | memory-lancedb |
|------|------------|----------------|
| 存储位置 | `memory/YYYY-MM-DD.md`（Markdown 文件） | LanceDB 向量数据库 |
| 存储方式 | LLM agent 主动写入（agentic flush） | `agent_end` hook 自动提取 + embedding |
| 触发时机 | compaction 前（token/transcript 阈值） | 每次 agent run 结束 |
| 召回工具 | `memory_search` + `memory_get` | `memory_recall` |
| 检索方式 | 混合检索（向量 + BM25 + MMR + 时间衰减） | 纯向量相似度 |
| 可读性 | 人可读的 Markdown 文件 | 二进制数据库 |
| 安全约束 | flush prompt 约束只写 memory/ 目录，不覆盖 MEMORY.md/SOUL.md 等 | 去重（>0.95 跳过） |

---

## 五、时间衰减（Temporal Decay）

源码：`extensions/memory-core/src/memory/temporal-decay.ts`

### 5.1 原理

使用经典的**指数衰减（Exponential Decay）**模型，即放射性衰变的半衰期公式：

```
decayedScore = score × e^(-λ × ageInDays)
```

其中：
- `λ = ln(2) / halfLifeDays`（衰减常数）
- `halfLifeDays` = 半衰期，默认 30 天

含义：每经过 `halfLifeDays` 天，记忆的检索得分减半。例如半衰期 30 天时，60 天前的记忆得分只剩 25%，90 天前只剩 12.5%。

### 5.2 默认配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `enabled` | `false` | 默认关闭 |
| `halfLifeDays` | `30` | 半衰期天数 |

### 5.3 时间戳提取优先级

对每条检索结果，按以下优先级确定其"年龄"：

1. **路径日期解析**：从文件路径提取 `memory/YYYY-MM-DD.md` 中的日期作为记忆创建时间
2. **Evergreen 豁免**：`MEMORY.md` 和 `memory/` 下的非日期文件（如 `memory/preferences.md`）不衰减，视为长期知识，返回 `null`（跳过衰减）
3. **文件系统 mtime 回退**：无法从路径提取日期时，使用 `fs.stat` 的 `mtime` 作为近似时间

### 5.4 目的

- 避免过时的记忆（如旧的项目决策、已完成的 todo）在检索中占据高位
- 让 LLM 优先引用近期的、仍然有效的记忆
- 保护常青知识（`MEMORY.md`、主题文件）不受时间惩罚

---

## 六、Citation 装饰

源码：`extensions/memory-core/src/tools.citations.ts`

### 6.1 原理

在 `memory_search` 返回的每条结果的 `snippet` 末尾追加来源标注，格式为：

```
<原始 snippet 内容>

Source: memory/2025-03-15.md#L10-L25
```

citation 格式规则：
- 单行结果：`<path>#L<lineNumber>`（如 `memory/2025-03-15.md#L10`）
- 多行结果：`<path>#L<startLine>-L<endLine>`（如 `memory/2025-03-15.md#L10-L25`）

### 6.2 是否包含 Citation 的决策

通过 `shouldIncludeCitations()` 判断，受 `memory.citations` 配置和会话类型控制：

| `memory.citations` 配置 | 行为 |
|--------------------------|------|
| `"on"` | 始终包含 citation |
| `"off"` | 始终不包含 citation |
| `"auto"`（默认） | 仅 **direct 私聊** 包含，group/channel 不包含 |

`auto` 模式下通过解析 sessionKey 判断聊天类型：包含 `group` 或 `channel` token 的会话不输出 citation，避免在群聊中泄露内部文件路径。

### 6.3 QMD 后端的字符预算截断

当使用 QMD 后端时，`clampResultsByInjectedChars()` 按字符预算（`maxInjectedChars`）截断结果：

1. 按顺序遍历结果列表
2. 如果完整 snippet 在预算内，保留整条
3. 如果超出预算，截断当前 snippet 的剩余部分，停止
4. 目的是控制注入到 prompt 的总 token 量

### 6.4 目的

- 让 LLM 在回复中引用记忆来源，用户可验证（`Source: memory/2025-03-15.md#L10-L25`）
- auto 模式在群聊中自动关闭，保护隐私
- QMD 后端的字符预算防止召回过多内容撑爆上下文窗口

---

## 七、关键数据流总结

```
用户消息
  │
  ├─ System Prompt 构建阶段
  │   └─ buildMemorySection() → 注入 "## Memory Recall" guidance
  │
  ├─ LLM 推理阶段
  │   └─ LLM 根据 guidance 调用 memory_search / memory_get
  │       └─ Tool result 返回记忆片段到对话上下文
  │
  └─ 存储阶段（memory-core 路径）
      │
      ├─ Token 接近 compaction 阈值
      │   └─ shouldRunMemoryFlush() → true
      │
      ├─ 生成 flush plan（target: memory/YYYY-MM-DD.md）
      │
      ├─ 启动独立 agent run（trigger: "memory"）
      │   ├─ User prompt: "Pre-compaction memory flush..."
      │   └─ System prompt: "Pre-compaction memory flush turn..."
      │
      ├─ LLM 分析对话 → write/edit tool → 写入 memory/YYYY-MM-DD.md
      │
      └─ Search manager 重新索引 memory/ 文件
```



- 独立 agent run
- pre compaction hook?
- 召回：MMR 重排序、时间衰减、citation 装饰(原始内容)




## OpenClaw Built-in Memory Research

本节基于 OpenClaw 官方文档对其内置 memory 的功能与接入方式进行调研，用于明确 `smem-openclaw` 替换 `memory` slot 后需要承担的职责边界。

### Built-in Memory Features

OpenClaw 内置 memory 的核心目标，是为 Agent 提供一套默认的长期记忆机制。当前已知能力包括：

1. 默认以工作区 Markdown 文件作为 memory source of truth。
   - `MEMORY.md`：长期、整理后的记忆。
   - `memory/YYYY-MM-DD.md`：按天记录的追加式 memory。
2. 提供自动或显式的 memory recall 能力。
   - active memory plugin 会在 prompt 构建阶段参与 memory 注入。
   - 也可通过 memory 相关 tool 做显式搜索或读取。
3. 提供 memory search / retrieval backend。
   - 默认实现围绕 Markdown 文件、embedding、向量索引、SQLite/QMD 等本地能力展开。
4. 支持自动 memory flush / compaction 前记忆写入提醒。
   - 用于在上下文压缩前，把持久化价值较高的信息写入 memory。
5. memory plugin 通过 slot 机制排他启用。
   - `plugins.slots.memory` 用于指定当前唯一生效的 memory plugin。

### OpenClaw Extension Points

OpenClaw 的 memory 相关实现不是单一接口，而是由 slot、tool、hook 和若干补充扩展点共同组成。本项目只接入对替换 `memory` slot 必要的部分。

#### Hooks

hooks 适合承载每轮、事件驱动的逻辑：

1. `before_prompt_build`
   - 可在 prompt 构建前执行动态 recall 或注入上下文。
   - 本项目不把它作为默认 recall 主路径，只保留作兼容/备选模式。
2. `agent_end`
   - 可在一轮对话结束后执行自动 capture / store。
   - 本项目第一版使用它作为自动存储主路径。

#### Memory Capability

memory capability 用于向 OpenClaw 注册一组 memory 相关能力描述，典型内容包括：

1. prompt builder
   - 为 memory 系统追加静态或半静态的 prompt section。
   - 更适合描述“memory 如何被使用”“有哪些 memory tool 可用”“引用格式如何约束”等规则。
2. flush plan resolver
   - 为 compaction / memory flush 场景提供计划。
   - 更适合上下文压缩前的持久化策略，而不是每轮实时 recall。
3. runtime / artifacts
   - 用于向系统暴露 memory runtime 或 public artifacts。
   - 更适合做 memory backend 状态、管理接口、可浏览对象等扩展。

对第三方 memory plugin 来说，memory capability 更像“补充声明”层：

1. 它适合补 memory 使用说明、compaction flush 策略、artifact 暴露。
2. 它不适合作为本项目 recall/store 的主链路。
3. 本项目第一版不实现 capability，只将其视为后续增强点。

#### Memory Runtime

memory runtime 更偏运行时 backend 接口，用于告诉 OpenClaw：当前 active memory plugin 如何提供底层 memory manager / backend 配置。

它主要解决的问题包括：

1. 返回 memory search manager
   - 供系统或其他 memory 相关流程拿到当前 memory backend 的检索管理器。
2. 解析当前 memory backend 配置
   - 例如 builtin、qmd 或其他 backend 的运行时配置。
3. 生命周期管理
   - 如关闭、回收、统一管理多个 memory manager 实例。

这个接口更接近“memory 后端适配层”，而不是“对话轮次 hook”。

对于 `smem-openclaw` 而言：

1. 如果未来要深度接入 OpenClaw memory runtime 生态，可以考虑实现 runtime。
2. 第一版 recall/store 都委托给 `smem server`，客户端不需要再在本地维护 memory search manager。
3. 因此第一版不实现 runtime。

#### Memory Prompt Section

memory prompt section 用于向系统 prompt 中追加 memory 使用说明，它关注的是“模型该如何理解和使用记忆”，而不是“去哪里查记忆”。

它通常适合承载以下内容：

1. 告诉模型当前存在长期记忆上下文。
2. 约束模型如何引用 memory。
3. 提醒模型不要盲信过时或弱相关 memory。
4. 说明 memory tool 的用途和边界。

这类能力通常是静态规则补充，优点是：

1. 能让模型更稳定地使用注入的 memory。
2. 能减少模型把 memory 误当作绝对事实的风险。

它不能替代动态 recall，也不负责实际存储。因此对本项目而言，它属于推荐增强项，不是第一版必须实现项。

### Recall Strategy Options

围绕 recall，本项目目前识别出两种可行方案：

1. hook 自动注入型
2. tool 驱动型

两者的核心差别在于：recall 是由插件在 prompt 构建前主动完成，还是由模型在需要时通过 tool 主动触发。

#### Option A: Hook-based Recall

hook 自动注入型，即当前文档前半部分默认描述的方案：

1. 在 `before_prompt_build` 中读取当前 prompt / messages。
2. 插件主动请求 `POST /api/v1/memories/recall`。
3. 将结果格式化后直接注入 prompt。
4. `agent_end` 仍负责自动 store。

优点：

1. recall 主链路稳定。
   - 不依赖模型是否记得调用 tool。
2. 对小模型更友好。
   - 即使模型 tool use 能力较弱，也能稳定拿到 memory。
3. 实现路径直接。
   - 与 `before_prompt_build` / `agent_end` 设计天然一致。

缺点：

1. 容易过度注入。
   - 每轮都 recall 并注入，可能带来额外 token 开销。
2. recall 查询不够精细。
   - 通常只能基于当前 prompt 或 messages 直接生成查询，灵活度不如模型按需构造检索词。
3. 与 OpenClaw 默认 memory 体验不完全一致。
   - 默认 memory 更偏向提供 `memory_search` / `memory_get` 工具，由模型自主决定是否使用。

#### Option B: Tool-based Recall

tool 驱动型 recall，即插件注册 `memory_search`、`memory_get` 等 tool，并在 system prompt 中说明模型需要按需使用这些 tool。

典型流程：

1. `smem-openclaw` 占用 `memory` slot。
2. 插件向 OpenClaw 注册 memory 相关 tool。
3. system prompt 或 memory prompt section 告诉模型：当需要长期记忆时，应优先使用 `memory_search`。
4. 模型根据当前任务，自主决定是否调用 tool、查询什么内容、是否继续调用 `memory_get` 读取更完整上下文。
5. `agent_end` 仍可继续承担自动 store，或后续再补显式 `memory_store` tool。

优点：

1. 更贴近 OpenClaw 默认 memory 生态。
   - 默认 memory plugin 的核心体验就是 memory tool + system prompt 引导。
2. 模型主动性更强。
   - 模型可以按需搜索，而不是每轮都被动接收 recall 注入。
3. token 使用更节制。
   - 不需要时不搜索，不会每轮都塞入 memory 内容。
4. recall 查询通常更灵活。
   - 模型可以围绕当前问题构造更精确的 query。
5. 更容易保持“替换 slot 后体验不降级”。
   - 用户仍然保有 memory tool 能力，而不是切换成完全不同的 recall 交互模型。

缺点：

1. recall 命中依赖模型行为。
   - 如果模型忘记调用 tool，或 tool use 能力较弱，则 memory 可能没有被使用。
2. 需要更强的 tool 设计。
   - 不只是一个 recall API client，还要考虑 tool schema、tool 返回格式、system prompt 引导语。
3. 首版实现复杂度更高。
   - 需要补 memory tool，而不只是 hook 注入。

#### Comparison Summary

1. hook 自动注入型
   - 更像“插件代替模型提前查好并塞进 prompt”。
   - 优先保证 recall 一定发生。
2. tool 驱动型
   - 更像“插件提供检索能力，模型在需要时主动查”。
   - 优先保持 OpenClaw memory 的原生使用体验。

#### Recommended Direction

从“替换 OpenClaw memory slot”的目标看，tool 驱动型 recall 更符合 OpenClaw 原生 memory 的设计习惯：

1. active memory plugin 不只是自动注入上下文，还应提供 memory tool 能力。
2. 默认 memory 体验是通过 system prompt 引导模型使用 `memory_search` / `memory_get`。
3. 如果 `smem-openclaw` 完全替换 slot，但不提供 tool，用户会明显感觉 memory 能力退化。

因此，当前建议调整为：

1. recall 主路径优先考虑 tool 驱动型。
   - `memory_search` 对接 `POST /api/v1/memories/recall`。
   - 如有需要，后续再补 `memory_get` 或其他 memory tool。
2. store 仍保留 `agent_end` 自动 capture。
   - 这是一个与 recall 正交的能力，不受 recall 方案影响。
3. hook 自动注入型 recall 可作为备选或兼容模式。
   - 当用户明确希望“每轮自动带入 memory”时，可再启用。
