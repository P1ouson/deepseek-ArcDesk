import { describe, expect, it } from "vitest";
import { shouldOpenNewBrowserTab } from "./previewBrowserTabPolicy";

describe("shouldOpenNewBrowserTab", () => {
  it("opens first tab when none exist", () => {
    expect(shouldOpenNewBrowserTab(false)).toBe(true);
  });

  it("reuses existing tabs when switching back to browser", () => {
    expect(shouldOpenNewBrowserTab(true)).toBe(false);
  });

  it("forces a new tab from the add menu", () => {
    expect(shouldOpenNewBrowserTab(true, { forceNewTab: true })).toBe(true);
  });
});
