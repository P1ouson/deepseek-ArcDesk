import { describe, expect, it } from "vitest";
import { buildTimelineRows, type ToolItem } from "./actionStream";
import type { Item } from "./useController";

function assistant(id: string, text = "", reasoning = "", streaming = false): Item {
  return { kind: "assistant", id, text, reasoning, streaming };
}

function tool(id: string): ToolItem {
  return {
    kind: "tool",
    id,
    name: "grep",
    args: "{}",
    readOnly: true,
    status: "done",
    output: "src/foo.ts:1:match",
  };
}

describe("buildTimelineRows reasoning coalesce", () => {
  it("merges reasoning-only assistants separated by tool segments", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "", "first thought"),
      tool("t1"),
      assistant("a2", "", "second thought"),
      tool("t2"),
      assistant("a3", "done", "final thought"),
    ];

    const rows = buildTimelineRows(items, new Map());

    const assistants = rows.filter((r) => r.kind === "single" && r.item.kind === "assistant");
    expect(assistants).toHaveLength(2);

    const merged = assistants[0]!;
    expect(merged.kind).toBe("single");
    if (merged.kind !== "single" || merged.item.kind !== "assistant") throw new Error("expected assistant");
    expect(merged.item.text).toBe("");
    expect(merged.item.reasoning).toBe("first thought\n\nsecond thought\n\nfinal thought");

    const answer = assistants[1]!;
    if (answer.kind !== "single" || answer.item.kind !== "assistant") throw new Error("expected assistant");
    expect(answer.item.text).toBe("done");
    expect(answer.item.reasoning).toBe("");
  });

  it("places merged reasoning before tool segments in the turn", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "", "thinking"),
      tool("t1"),
      assistant("a2", "answer"),
    ];

    const rows = buildTimelineRows(items, new Map());
    expect(rows.map((r) => (r.kind === "action-segment" ? "tools" : r.item.kind))).toEqual([
      "user",
      "assistant",
      "tools",
      "assistant",
    ]);
  });

  it("merges many consecutive reasoning-only blocks after interim narration", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "fix it" },
      assistant("a0", "store.ts 里 listTasks 未按 status 过滤"),
      assistant("a1", "", "thought 1"),
      assistant("a2", "", "thought 2"),
      assistant("a3", "", "thought 3"),
      assistant("a4", "", "thought 4"),
      assistant("a5", "final answer"),
    ];

    const rows = buildTimelineRows(items, new Map());
    const thinking = rows.filter(
      (r) => r.kind === "single" && r.item.kind === "assistant" && r.item.reasoning.trim() && !r.item.text.trim(),
    );
    expect(thinking).toHaveLength(1);
    if (thinking[0]?.kind !== "single" || thinking[0].item.kind !== "assistant") throw new Error("expected assistant");
    expect(thinking[0].item.reasoning).toBe("thought 1\n\nthought 2\n\nthought 3\n\nthought 4");
  });

  it("keeps live stream attached to merged reasoning block", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "", "part one"),
      tool("t1"),
      assistant("a2", "", "part two", true),
    ];

    const rows = buildTimelineRows(items, new Map(), { id: "a2", text: "", reasoning: "part two live" });
    const merged = rows.find((r) => r.kind === "single" && r.item.kind === "assistant" && !r.item.text.trim());
    expect(merged?.kind).toBe("single");
    if (merged?.kind !== "single" || merged.item.kind !== "assistant") throw new Error("expected merged assistant");
    expect(merged.item.id).toBe("a2");
    expect(merged.item.reasoning).toBe("part one\n\npart two live");
    expect(merged.item.streaming).toBe(true);
  });
});
