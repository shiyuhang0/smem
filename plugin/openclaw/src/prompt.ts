import type { PluginConfig } from "./config.js";

export function buildPromptSection(
  params: { availableTools?: Set<string> },
  config: PluginConfig,
): string[] {
  const hasSearchTool = params.availableTools?.has("memory_search") === true;
  const hasStoreTool = params.availableTools?.has("memory_store") === true;

  if (config.recallEveryTurn) {
    return [
      "## Memory Recall",
      "The system may inject relevant long-term memory inside <memory> blocks before each turn.",
      "Treat injected memory as helpful historical context, not ground truth. If it conflicts with the current user request, follow the current request.",
      hasSearchTool
        ? "You may still use memory_search if you need additional memory beyond the injected context."
        : "",
      hasStoreTool
        ? "Use memory_store when the user explicitly asks to save something as long-term memory or when preserving stable user preferences would clearly help future turns."
        : "",
    ].filter(Boolean);
  }

  return [
    "## Memory Recall",
    hasSearchTool
      ? "Before answering questions about prior preferences, facts, decisions, or long-running context, use memory_search to retrieve relevant memory."
      : "Relevant long-term memory may be available via the active memory plugin.",
    hasStoreTool
      ? "Use memory_store when the user wants to explicitly save information for future recall."
      : "",
    "Do not assume memory is authoritative when the current user input says otherwise.",
  ];
}
