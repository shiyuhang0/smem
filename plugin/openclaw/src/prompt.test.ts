import { describe, expect, it } from "vitest";

import { buildPromptSection } from "./prompt.js";

describe("buildPromptSection", () => {
  it("encourages tool usage in tool mode", () => {
    const section = buildPromptSection(
      { availableTools: new Set(["memory_search", "memory_store"]) },
      { toolMode: true, serverURL: "http://localhost:5173", topK: 5, storeMode: "smart", timeoutMs: 8000 },
    ).join("\n");

    expect(section).toContain("use memory_search to retrieve relevant memory");
    expect(section).toContain("Use memory_store when the user wants to explicitly save information for future recall.");
  });

  it("describes automatic mode without encouraging proactive tool use", () => {
    const section = buildPromptSection(
      { availableTools: new Set(["memory_search", "memory_store"]) },
      { toolMode: false, serverURL: "http://localhost:5173", topK: 5, storeMode: "smart", timeoutMs: 8000 },
    ).join("\n");

    expect(section).toContain(
      "may inject relevant long-term memory inside <memory> blocks before each turn and may store conversation memory automatically after each turn",
    );
    expect(section).toContain("usually does not need to be called proactively while automatic memory mode is active");
    expect(section).not.toContain("Before answering questions about prior preferences");
  });
});
