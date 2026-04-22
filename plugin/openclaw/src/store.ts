import { createSmemClient } from "./client.js";
import { resolveStoreContext } from "./context.js";

export async function storeTurn(params: {
  pluginConfig: unknown;
  content: string;
  ctx: {
    agentId?: string;
    sessionId?: string;
    sessionKey?: string;
    channelId?: string;
    trigger?: string;
  };
}) {
  await storeMemory(params);
}

export async function storeMemory(params: {
  pluginConfig: unknown;
  content: string;
  ctx: {
    agentId?: string;
    sessionId?: string;
    sessionKey?: string;
    channelId?: string;
    trigger?: string;
  };
}) {
  const client = createSmemClient(params.pluginConfig);
  const storeContext = resolveStoreContext(params.ctx);
  return await client.createMemory({
    content: params.content,
    mode: client.config.storeMode,
    agent_id: storeContext.agentID,
    session_id: storeContext.sessionID,
    source: storeContext.source,
    metadata: storeContext.metadata,
  });
}
