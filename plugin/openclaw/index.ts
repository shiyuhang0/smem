import { createMemorySearchTool, createMemoryStoreTool } from "./src/tools.js";
import { resolvePluginConfig } from "./src/config.js";
import { registerPluginHooks } from "./src/hooks.js";
import { buildPromptSection } from "./src/prompt.js";

export default {
  id: "smem-openclaw",
  name: "SMEM OpenClaw",
  description: "SMEM-backed memory plugin for OpenClaw",
  kind: "memory",
  register(api: any) {
    api.registerMemoryCapability({
      promptBuilder: (params: any) => buildPromptSection(params, resolvePluginConfig(api.pluginConfig)),
    });

    api.registerTool(
      (ctx: any) => createMemorySearchTool({ api, ctx }),
      { names: ["memory_search"] },
    );

    api.registerTool(
      (ctx: any) => createMemoryStoreTool({ api, ctx }),
      { names: ["memory_store"] },
    );

    registerPluginHooks(api);
  },
};
