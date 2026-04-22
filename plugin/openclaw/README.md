# smem-openclaw

OpenClaw memory plugin for `smem`.

## Features

- Default recall mode is tool-driven via `memory_search`.
- When `recallEveryTurn` is `true`, the plugin also performs hook-based recall in `before_prompt_build` and injects results inside `<memory>` blocks.
- Store runs automatically on `agent_end`.
- Provides `memory_store` for explicit long-term memory writes.

## OpenClaw Integration

- Declares `kind: "memory"` and occupies `plugins.slots.memory`.
- Uses `registerMemoryCapability({ promptBuilder })` to inject static memory guidance.
- Uses `registerTool(...)` to register `memory_search` and `memory_store`.
- Uses `api.on("before_prompt_build", ...)` for optional hook-based recall injection.
- Uses `api.on("agent_end", ...)` for automatic store.

## Tools

### `memory_search`

- Default recall path.
- Calls `POST /api/v1/memories/recall`.
- Intended for prior preferences, decisions, facts, and running context.

### `memory_store`

- Explicit store path.
- Calls `POST /api/v1/memories`.
- Intended for cases where the user explicitly asks to save something or the model decides a stable preference/fact should be persisted.

## Config

- `serverURL`: SMEM server base URL. Default: `http://localhost:5173`
- `recallEveryTurn`: enable hook-based recall injection. Default: `false`
- `topK`: recall result count. Default: `5`
- `storeMode`: `normal` or `smart`. Default: `smart`
- `timeoutMs`: request timeout in milliseconds. Default: `8000`

## OpenClaw Config Example

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
          "recallEveryTurn": false,
          "topK": 5,
          "storeMode": "smart",
          "timeoutMs": 8000
        }
      }
    }
  }
}
```

## Local Development

1. Install dependencies:

```bash
cd plugin/openclaw
npm install
```

2. Verify the package:

```bash
npm run build
npm run test
```

3. Load it in OpenClaw using a local path:

```json
{
  "plugins": {
    "enabled": true,
    "load": {
      "paths": ["/absolute/path/to/smem/plugin/openclaw"]
    },
    "slots": {
      "memory": "smem-openclaw"
    },
    "entries": {
      "smem-openclaw": {
        "enabled": true,
        "config": {
          "serverURL": "http://localhost:5173",
          "recallEveryTurn": false,
          "topK": 5,
          "storeMode": "smart",
          "timeoutMs": 8000
        }
      }
    }
  }
}
```

4. Restart OpenClaw Gateway after config changes.

## Publish And Install

### Option 1: Local install into OpenClaw

Use OpenClaw's local plugin install flow:

```bash
openclaw plugins install /absolute/path/to/smem/plugin/openclaw
```

For development symlink mode:

```bash
openclaw plugins install -l /absolute/path/to/smem/plugin/openclaw
```

Then set:

```json
{
  "plugins": {
    "slots": {
      "memory": "smem-openclaw"
    },
    "entries": {
      "smem-openclaw": {
        "enabled": true
      }
    }
  }
}
```

### Option 2: Publish to npm

1. Update `package.json` before publishing:
   - choose the final package name
   - remove or set `"private": false`
   - keep `openclaw.extensions = ["./index.ts"]`
2. Make sure the published package includes:
   - `index.ts`
   - `openclaw.plugin.json`
   - `src/**`
   - `package.json`
3. Publish:

```bash
cd plugin/openclaw
npm publish
```

4. Install from npm on an OpenClaw host:

```bash
openclaw plugins install <your-package-name>
```

5. Enable it in OpenClaw config and assign the memory slot.

## Runtime Requirements

- SMEM server must be running and reachable at `serverURL`.
- OpenClaw must load this plugin as the active `memory` slot.
- Gateway restart is required after plugin/config changes.
