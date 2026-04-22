import { createSmemClient } from "./client.js";
import { resolvePluginConfig } from "./config.js";
import { storeTurn } from "./store.js";
import { extractRecallQuery, extractStoreContent, formatRecallItems } from "./text.js";

export function registerPluginHooks(api: any) {
  api.on("before_prompt_build", async (event: any, ctx: any) => {
    const config = resolvePluginConfig(api.pluginConfig);
    if (!config.recallEveryTurn) {
      return;
    }

    const query = extractRecallQuery(event.prompt ?? "", Array.isArray(event.messages) ? event.messages : []);
    if (!query) {
      return;
    }

    try {
      const client = createSmemClient(api.pluginConfig);
      const items = await client.recall(query, config.topK);
      const prependContext = formatRecallItems(items);
      if (!prependContext) {
        return;
      }
      return { prependContext };
    } catch (error) {
      api.logger?.warn?.(
        `[smem-openclaw] recall skipped for ${ctx?.sessionKey ?? "unknown-session"}: ${formatError(error)}`,
      );
      return;
    }
  });

  api.on("agent_end", async (event: any, ctx: any) => {
    if (event?.success === false) {
      return;
    }

    const content = extractStoreContent(
      Array.isArray(event.messages) || typeof event.messages === "string" ? event.messages : [],
    );
    if (!content) {
      return;
    }

    api.logger?.info?.(
        `[smem-openclaw] message: ${event.messages} `
      );

    api.logger?.info?.(
        `[smem-openclaw] store content: ${content} `
      );

    try {
      await storeTurn({
        pluginConfig: api.pluginConfig,
        content,
        ctx,
      });
    } catch (error) {
      api.logger?.warn?.(
        `[smem-openclaw] store skipped for ${ctx?.sessionKey ?? "unknown-session"}: ${formatError(error)}`,
      );
    }
  });
}

function formatError(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
