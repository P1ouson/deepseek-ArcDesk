import { beforeEach, describe, expect, it } from "vitest";
import { loadProjectDrawerOpen, saveProjectDrawerOpen } from "./projectDrawerPrefs";

const store: Record<string, string> = {};

beforeEach(() => {
  for (const key of Object.keys(store)) delete store[key];
  const localStorage = {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
  };
  Object.defineProperty(globalThis, "window", {
    value: { localStorage },
    configurable: true,
  });
});

describe("projectDrawerPrefs", () => {
  it("migrates legacy arcdesk key to canonical ARCDESK key", () => {
    window.localStorage.setItem("arcdesk.studio.projectDrawerOpen", "0");
    window.localStorage.removeItem("ARCDESK.studio.projectDrawerOpen");
    expect(loadProjectDrawerOpen()).toBe(false);
    saveProjectDrawerOpen(true);
    expect(window.localStorage.getItem("ARCDESK.studio.projectDrawerOpen")).toBe("1");
    expect(window.localStorage.getItem("arcdesk.studio.projectDrawerOpen")).toBe("1");
  });
});
