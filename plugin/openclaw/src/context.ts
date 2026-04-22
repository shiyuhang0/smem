export type AgentContext = {
  agentId?: string;
  sessionId?: string;
  sessionKey?: string;
  workspaceDir?: string;
  modelProviderId?: string;
  modelId?: string;
  messageProvider?: string;
  trigger?: string;
  channelId?: string;
};

export type StoreContext = {
  agentID?: string;
  sessionID?: string;
  source?: string;
  metadata: Record<string, unknown>;
};

export function resolveStoreContext(ctx: AgentContext | undefined): StoreContext {
  const agentID = firstNonEmpty(ctx?.agentId, deriveAgentID(ctx?.sessionKey));
  const sessionID = firstNonEmpty(ctx?.sessionId, ctx?.sessionKey);
  const source = firstNonEmpty(ctx?.channelId, ctx?.trigger, ctx?.messageProvider);

  return {
    agentID,
    sessionID,
    source,
    metadata: compactRecord({
      plugin: "smem-openclaw",
      session_key: ctx?.sessionKey,
      workspace_dir: ctx?.workspaceDir,
      trigger: ctx?.trigger,
      channel_id: ctx?.channelId,
      message_provider: ctx?.messageProvider,
      model_provider_id: ctx?.modelProviderId,
      model_id: ctx?.modelId,
    }),
  };
}

function deriveAgentID(sessionKey?: string): string | undefined {
  if (!sessionKey) {
    return undefined;
  }
  const parts = sessionKey.split(":").filter(Boolean);
  if (parts.length >= 2) {
    return `${parts[0]}:${parts[1]}`;
  }
  return sessionKey;
}

function firstNonEmpty(...values: Array<string | undefined>): string | undefined {
  for (const value of values) {
    if (typeof value === "string" && value.trim()) {
      return value;
    }
  }
  return undefined;
}

function compactRecord(value: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(value).filter(([, item]) => item !== undefined && item !== ""));
}
