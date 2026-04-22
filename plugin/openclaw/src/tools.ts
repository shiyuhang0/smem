import { Type } from "@sinclair/typebox";

import { createSmemClient } from "./client.js";
import { storeMemory } from "./store.js";
import { formatToolResultText } from "./text.js";

const MemorySearchSchema = Type.Object({
  query: Type.String({ minLength: 1, description: "Memory search query" }),
  maxResults: Type.Optional(Type.Integer({ minimum: 1, maximum: 10 })),
});

const MemoryStoreSchema = Type.Object({
  content: Type.String({ minLength: 1, description: "Memory content to store" }),
});

export function createMemorySearchTool(params: { api: any; ctx: any }) {
  const client = createSmemClient(params.api.pluginConfig);

  return {
    name: "memory_search",
    description:
      "Searches SMEM for long-term memory related to prior preferences, decisions, facts, or ongoing context.",
    parameters: MemorySearchSchema,
    async execute(_toolCallId: string, rawParams: Record<string, unknown>) {
      const query = typeof rawParams.query === "string" ? rawParams.query.trim() : "";
      if (!query) {
        return textResult("Query is required.", {
          status: "invalid_request",
          items: [],
        });
      }

      const maxResults =
        typeof rawParams.maxResults === "number" && Number.isFinite(rawParams.maxResults)
          ? Math.max(1, Math.min(10, Math.trunc(rawParams.maxResults)))
          : client.config.topK;

      try {
        const items = await client.recall(query, maxResults);
        return textResult(formatToolResultText(items), {
          status: "ok",
          items,
          query,
          topK: maxResults,
          sessionKey: params.ctx?.sessionKey,
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        params.api.logger?.warn?.(`[smem-openclaw] memory_search failed: ${message}`);
        return textResult("Memory search is currently unavailable.", {
          status: "unavailable",
          error: message,
          items: [],
          query,
          topK: maxResults,
        });
      }
    },
  };
}

export function createMemoryStoreTool(params: { api: any; ctx: any }) {
  const client = createSmemClient(params.api.pluginConfig);

  return {
    name: "memory_store",
    description: "Stores a new long-term memory in SMEM for later recall.",
    parameters: MemoryStoreSchema,
    async execute(_toolCallId: string, rawParams: Record<string, unknown>) {
      const content = typeof rawParams.content === "string" ? rawParams.content.trim() : "";
      if (!content) {
        return textResult("Content is required.", {
          status: "invalid_request",
        });
      }

      try {
        const cleanedContent = content.replace(/<memory>[\s\S]*?<\/memory>/gi, " ").trim();
        if (!cleanedContent) {
          return textResult("Content is required.", {
            status: "invalid_request",
          });
        }

        const result = await storeMemory({
          pluginConfig: params.api.pluginConfig,
          content: cleanedContent,
          ctx: params.ctx ?? {},
        });
        return textResult("Memory stored successfully.", {
          status: "ok",
          content: cleanedContent,
          mode: client.config.storeMode,
          result,
          sessionKey: params.ctx?.sessionKey,
        });
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        params.api.logger?.warn?.(`[smem-openclaw] memory_store failed: ${message}`);
        return textResult("Memory store is currently unavailable.", {
          status: "unavailable",
          error: message,
          content,
          mode: client.config.storeMode,
        });
      }
    },
  };
}

function textResult(text: string, details: Record<string, unknown>) {
  return {
    content: [
      {
        type: "text",
        text,
      },
    ],
    details,
  };
}
