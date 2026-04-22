import { describe, expect, it } from "vitest";

import {
  extractRecallQuery,
  formatAgentEndTranscript,
  extractStoreContent,
  formatRecallItems,
  formatToolResultText,
  stripInjectedMemoryBlocks,
} from "./text.js";

describe("text helpers", () => {
  it("strips injected memory blocks from stored content", () => {
    expect(stripInjectedMemoryBlocks("hello <memory>old</memory> world")).toBe("hello world");
  });

  it("prefers latest user text for recall query and ignores memory block", () => {
    const query = extractRecallQuery("fallback", [
      { role: "assistant", content: "ignore" },
      { role: "user", content: "<memory>old</memory> new question" },
    ]);
    expect(query).toBe("new question");
  });

  it("formats recall items and store content minimally", () => {
    expect(
      formatRecallItems([{ memory: { content: "Remember this" }, score: 0.9 }]),
    ).toContain("Remember this");
    const toolText = formatToolResultText([
      {
        memory: { content: "Remember this", type: "fact", kinds: ["preference"] },
        score: 0.9,
        reason: "recently discussed",
      },
    ]);
    expect(toolText).toContain("Relevant memory results:");
    expect(toolText).toContain("1. Remember this");
    expect(toolText).toContain("type=fact | kinds=preference | score=0.900 | reason=recently discussed");
    expect(
      extractStoreContent([
        { role: "user", content: "one" },
        { role: "assistant", content: "skip" },
        { role: "user", content: "two" },
      ]),
    ).toBe("two");
  });

  it("reads alternate message shapes", () => {
    const query = extractRecallQuery("fallback", [
      { type: "human", body: "from body" },
      { author: { role: "user" }, parts: [{ text: "latest part text" }] },
    ]);
    expect(query).toBe("latest part text");
  });

  it("extracts only the last transcript segment from agent_end text", () => {
    const fence = "```";
    const transcript = [
      "A new session was started via /new or /reset.",
      "",
      "Sender (untrusted metadata):",
      "",
      fence + "json",
      '{ "label": "openclaw-control-ui", "id": "openclaw-control-ui" }',
      fence,
      "[Wed 2026-04-22 19:31 GMT+8] 我喜欢什么颜色",
      "",
      "Sender (untrusted metadata):",
      "",
      fence + "json",
      '{ "label": "openclaw-control-ui", "id": "openclaw-control-ui" }',
      fence,
      "[Wed 2026-04-22 19:33 GMT+8] 我喜欢晴天 ",
      "<system-reminder>",
      "ignored",
      "</system-reminder>",
    ].join("\n");

    expect(formatAgentEndTranscript(transcript)).toBe("我喜欢晴天");
    expect(extractStoreContent(transcript)).toBe("我喜欢晴天");
  });

  it("parses transcript logs even when wrapped in a user message", () => {
    const fence = "```";
    const transcript = [
      "Sender (untrusted metadata):",
      "",
      fence + "json",
      '{ "label": "openclaw-control-ui", "id": "openclaw-control-ui" }',
      fence,
      "[Wed 2026-04-22 19:31 GMT+8] 我喜欢什么颜色",
      "",
      "Sender (untrusted metadata):",
      "",
      fence + "json",
      '{ "label": "openclaw-control-ui", "id": "openclaw-control-ui" }',
      fence,
      "[Wed 2026-04-22 19:33 GMT+8] 我喜欢晴天",
      "<system-reminder>",
      "ignored",
      "</system-reminder>",
    ].join("\n");

    expect(
      extractStoreContent([{ role: "user", content: transcript }]),
    ).toBe("我喜欢晴天");
  });

  it("keeps only the last plain transcript line after cleaning startup text", () => {
    const transcript = `A new session was started via /new or /reset. Execute your Session Startup sequence now - read the required files before responding to the user.
Current time: Wednesday, April 22nd, 2026 - 6:59 PM (Asia/Shanghai) / 2026-04-22 10:59 UTC

你是什么模型

hi，输出模型

你是什么模型

我喜欢晴天

我家猫叫什么近k轮对话,k写死为1。
<system-reminder>
ignored
</system-reminder>`;

    expect(formatAgentEndTranscript(transcript)).toBe("我家猫叫什么");
    expect(extractStoreContent([{ role: "user", content: transcript }])).toBe("我家猫叫什么");
  });
});
