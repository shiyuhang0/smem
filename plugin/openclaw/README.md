# smem-openclaw

OpenClaw memory plugin for `smem`.

## Features

- `toolMode=true` is the default, using `memory_search` and `memory_store` as the primary memory path.
- `toolMode=false` switches to hook-based recall in `before_prompt_build` and automatic store on `agent_end`.
- `memory_search` and `memory_store` are always registered in both modes.

## OpenClaw Integration

- Declares `kind: "memory"` and occupies `plugins.slots.memory`.
- Uses `registerMemoryCapability({ promptBuilder })` to inject static memory guidance.
- Uses `registerTool(...)` to register `memory_search` and `memory_store`.
- Uses `api.on("before_prompt_build", ...)` for automatic hook-based recall when `toolMode` is `false`.
- Uses `api.on("agent_end", ...)` for automatic hook-based store when `toolMode` is `false`.

## Tools

### `memory_search`

- Manual recall path.
- Calls `POST /api/v1/memories/recall`.
- Intended for prior preferences, decisions, facts, and running context.

### `memory_store`

- Manual store path.
- Calls `POST /api/v1/memories`.
- Intended for cases where the user explicitly asks to save something or the model decides a stable preference/fact should be persisted.

## Config

- `serverURL`: SMEM server base URL. Default: `http://localhost:5173`
- `toolMode`: when `true`, use tools as the default recall/store path. When `false`, use hook-based automatic recall/store. Default: `true`
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
