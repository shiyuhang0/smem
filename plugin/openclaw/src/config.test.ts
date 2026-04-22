import { describe, expect, it } from "vitest";

import { resolvePluginConfig } from "./config.js";

describe("resolvePluginConfig", () => {
  it("defaults toolMode to true", () => {
    expect(resolvePluginConfig({}).toolMode).toBe(true);
  });

  it("accepts explicit false for automatic hook mode", () => {
    expect(resolvePluginConfig({ toolMode: false }).toolMode).toBe(false);
  });
});
