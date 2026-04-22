import { resolvePluginConfig, type PluginConfig } from "./config.js";

export type RecallItem = {
  memory?: {
    id?: string;
    content?: string;
    kinds?: string[];
    type?: string;
    metadata?: Record<string, unknown>;
    agent_id?: string;
    session_id?: string;
    source?: string;
  };
  score?: number;
  reason?: string;
};

export function createSmemClient(rawConfig: unknown) {
  const config = resolvePluginConfig(rawConfig);
  return {
    config,
    async recall(content: string, topK?: number): Promise<RecallItem[]> {
      const response = await postJson(config, "/api/v1/memories/recall", {
        content,
        top_k: topK ?? config.topK,
      });
      return Array.isArray(response?.items) ? response.items : [];
    },
    async createMemory(payload: Record<string, unknown>): Promise<unknown> {
      return await postJson(config, "/api/v1/memories", payload);
    },
  };
}

async function postJson(config: PluginConfig, path: string, body: Record<string, unknown>) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), config.timeoutMs);
  try {
    const response = await fetch(new URL(path, ensureTrailingSlash(config.serverURL)).toString(), {
      method: "POST",
      headers: {
        "content-type": "application/json",
      },
      body: JSON.stringify(body),
      signal: controller.signal,
    });

    if (!response.ok) {
      const text = await response.text();
      throw new Error(`smem request failed: ${response.status} ${text}`);
    }

    return await response.json();
  } finally {
    clearTimeout(timeout);
  }
}

function ensureTrailingSlash(serverURL: string): string {
  return serverURL.endsWith("/") ? serverURL : `${serverURL}/`;
}
