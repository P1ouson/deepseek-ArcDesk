import { describe, expect, it } from "vitest";
import type { ModelInfo } from "./types";
import {
  isPendingConfirmed,
  mergeSelectedCurrent,
  resolveModelDisplayLabel,
  resolvePendingRef,
} from "./modelSwitcherState";

describe("mergeSelectedCurrent", () => {
  const models: ModelInfo[] = [
    { ref: "deepseek-flash/deepseek-v4-flash", provider: "deepseek-flash", model: "deepseek-v4-flash", current: true },
    { ref: "deepseek-pro/deepseek-v4-pro", provider: "deepseek-pro", model: "deepseek-v4-pro", current: false },
  ];

  it("marks the selected model SKU even when API current differs", () => {
    const merged = mergeSelectedCurrent(models, "deepseek-pro/deepseek-v4-pro");
    expect(merged.find((m) => m.model === "deepseek-v4-pro")?.current).toBe(true);
    expect(merged.find((m) => m.model === "deepseek-v4-flash")?.current).toBe(false);
  });
});

describe("isPendingConfirmed", () => {
  const models: ModelInfo[] = [
    { ref: "deepseek-flash/deepseek-v4-pro", provider: "deepseek-flash", model: "deepseek-v4-pro", current: true },
    { ref: "deepseek-flash/deepseek-v4-flash", provider: "deepseek-flash", model: "deepseek-v4-flash", current: false },
  ];

  it("matches by model SKU rather than exact ref", () => {
    expect(isPendingConfirmed(models, "deepseek-pro/deepseek-v4-pro")).toBe(true);
    expect(resolvePendingRef(models, "deepseek-pro/deepseek-v4-pro")).toBe("deepseek-flash/deepseek-v4-pro");
  });
});

describe("resolveModelDisplayLabel", () => {
  const models: ModelInfo[] = [
    { ref: "deepseek-flash/deepseek-v4-flash", provider: "deepseek-flash", model: "deepseek-v4-flash", current: true },
    { ref: "deepseek-pro/deepseek-v4-pro", provider: "deepseek-pro", model: "deepseek-v4-pro", current: false },
  ];

  it("prefers the user selection over stale API current", () => {
    expect(resolveModelDisplayLabel(models, "", "deepseek-pro/deepseek-v4-pro")).toBe("deepseek-v4-pro");
  });

  it("prefers backend meta over stale API current", () => {
    expect(resolveModelDisplayLabel(models, "deepseek-v4-pro", "")).toBe("deepseek-v4-pro");
  });

  it("falls back to meta label", () => {
    expect(resolveModelDisplayLabel([], "deepseek/deepseek-v4-flash", "")).toBe("deepseek-v4-flash");
  });
});
