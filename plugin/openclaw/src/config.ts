const DEFAULT_SERVER_URL = "http://localhost:5173";
const DEFAULT_TOP_K = 5;
const DEFAULT_TIMEOUT_MS = 8000;

export type PluginConfig = {
  serverURL: string;
  recallEveryTurn: boolean;
  topK: number;
  storeMode: "normal" | "smart";
  timeoutMs: number;
};

export function resolvePluginConfig(raw: unknown): PluginConfig {
  const value = isRecord(raw) ? raw : {};
  return {
    serverURL: typeof value.serverURL === "string" && value.serverURL.trim() ? value.serverURL.trim() : DEFAULT_SERVER_URL,
    recallEveryTurn: value.recallEveryTurn === true,
    topK: clampInteger(value.topK, DEFAULT_TOP_K, 1, 10),
    storeMode: value.storeMode === "normal" ? "normal" : "smart",
    timeoutMs: clampInteger(value.timeoutMs, DEFAULT_TIMEOUT_MS, 1000, 120000),
  };
}

function clampInteger(value: unknown, fallback: number, min: number, max: number): number {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(max, Math.max(min, Math.trunc(value)));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
