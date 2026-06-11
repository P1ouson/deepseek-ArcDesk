import { describe, expect, it } from "vitest";
import { controllerInitialState } from "./useController";
import {
  listTabAttention,
  openTabsBarItems,
  tabIsAgentRunning,
  tabNeedsDecision,
  tabPulseForState,
} from "./tabSessionActivity";
import type { TabMeta } from "./types";

const meta = (id: string, title: string): TabMeta => ({
  id,
  scope: "project",
  workspaceRoot: "/proj",
  workspaceName: "proj",
  topicId: `topic-${id}`,
  topicTitle: title,
  label: "model",
  ready: true,
  running: false,
  mode: "normal",
  active: id === "a",
  cwd: "/proj",
});

describe("tabSessionActivity", () => {
  it("detects running and decision state", () => {
    expect(tabIsAgentRunning({ running: true, turnActive: false })).toBe(true);
    expect(tabNeedsDecision({ approval: { id: "1", tool: "bash", subject: "" } })).toBe(true);
  });

  it("maps pulse colors: green idle, yellow running", () => {
    expect(tabPulseForState({ ...controllerInitialState, running: true, turnActive: false })).toBe("running");
    expect(tabPulseForState(controllerInitialState)).toBe("completed");
    expect(tabPulseForState(undefined, false)).toBe("completed");
  });

  it("lists attention across open tabs", () => {
    const states = new Map([
      ["a", { ...controllerInitialState, running: true }],
      ["b", { ...controllerInitialState, approval: { id: "2", tool: "write_file", subject: "x" } }],
    ]);
    const rows = listTabAttention([meta("a", "Alpha"), meta("b", "Beta")], states, {
      plan: "plan",
      approval: (tool) => tool,
      ask: "ask",
    });
    expect(rows[0]?.pulse).toBe("running");
    expect(rows[1]?.needsDecision).toBe(true);
    expect(rows[1]?.pulse).toBe("completed");
  });

  it("defaults idle tabs to green pulse", () => {
    const rows = listTabAttention([meta("a", "Alpha")], new Map(), {
      plan: "plan",
      approval: (tool) => tool,
      ask: "ask",
    });
    expect(rows[0]?.pulse).toBe("completed");
  });

  it("lists every open tab for the tab bar", () => {
    const rows = listTabAttention(
      [meta("a", "Alpha"), meta("b", "Beta"), meta("c", "Gamma")],
      new Map(),
      { plan: "plan", approval: (tool) => tool, ask: "ask" },
    );
    expect(openTabsBarItems(rows)).toHaveLength(3);
    expect(openTabsBarItems(rows).map((row) => row.tabId)).toEqual(["a", "b", "c"]);
  });
});
