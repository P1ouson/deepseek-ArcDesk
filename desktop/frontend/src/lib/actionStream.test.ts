import { describe, expect, it } from "vitest";
import { buildTimelineRows, type ToolItem } from "./actionStream";
import type { Item } from "./useController";

function assistant(id: string, text = "", reasoning = "", streaming = false): Item {
  return { kind: "assistant", id, text, reasoning, streaming };
}

function tool(id: string, status: ToolItem["status"] = "done"): ToolItem {
  return {
    kind: "tool",
    id,
    name: "grep",
    args: "{}",
    readOnly: true,
    status,
    output: "src/foo.ts:1:match",
  };
}

function rowKinds(rows: ReturnType<typeof buildTimelineRows>): string[] {
  return rows.map((r) => {
    if (r.kind === "thinking-block") return "thinking";
    if (r.kind === "action-segment") return "tools";
    return r.item.kind;
  });
}

function bashTool(id: string, command: string, status: ToolItem["status"] = "done", output = ""): ToolItem {
  return {
    kind: "tool",
    id,
    name: "bash",
    args: JSON.stringify({ command }),
    readOnly: false,
    status,
    output,
    isShell: true,
  };
}

describe("buildTimelineRows thinking blocks", () => {
  it("folds reasoning and tools into one thinking block before the answer", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "", "first thought"),
      tool("t1"),
      assistant("a2", "", "second thought"),
      tool("t2"),
      assistant("a3", "done", "final thought"),
    ];

    const rows = buildTimelineRows(items, new Map());
    expect(rowKinds(rows)).toEqual(["user", "thinking", "assistant"]);

    const block = rows[1];
    expect(block?.kind).toBe("thinking-block");
    if (block?.kind !== "thinking-block") throw new Error("expected thinking block");
    expect(block.block.reasoning).toBe("first thought\n\nsecond thought\n\nfinal thought");
    expect(block.block.entries).toHaveLength(2);
    expect(block.block.complete).toBe(true);

    const answer = rows[2];
    expect(answer?.kind).toBe("single");
    if (answer?.kind !== "single" || answer.item.kind !== "assistant") throw new Error("expected assistant");
    expect(answer.item.text).toBe("done");
    expect(answer.item.reasoning).toBe("");
  });

  it("places thinking block before the answer instead of separate tool rows", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "", "thinking"),
      tool("t1"),
      assistant("a2", "answer"),
    ];

    const rows = buildTimelineRows(items, new Map());
    expect(rowKinds(rows)).toEqual(["user", "thinking", "assistant"]);
  });

  it("keeps interim narration separate from later thinking blocks", () => {
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
    expect(rowKinds(rows)).toEqual(["user", "assistant", "thinking", "assistant"]);

    const block = rows[2];
    expect(block?.kind).toBe("thinking-block");
    if (block?.kind !== "thinking-block") throw new Error("expected thinking block");
    expect(block.block.reasoning).toBe("thought 1\n\nthought 2\n\nthought 3\n\nthought 4");
  });

  it("keeps live stream attached to the thinking block", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "", "part one"),
      tool("t1"),
      assistant("a2", "", "part two", true),
    ];

    const rows = buildTimelineRows(items, new Map(), { id: "a2", text: "", reasoning: "part two live" });
    const block = rows.find((r) => r.kind === "thinking-block");
    expect(block?.kind).toBe("thinking-block");
    if (block?.kind !== "thinking-block") throw new Error("expected thinking block");
    expect(block.block.id).toBe("a2");
    expect(block.block.reasoning).toBe("part one\n\npart two live");
    expect(block.block.streaming).toBe(true);
    expect(block.block.complete).toBe(false);
  });

  it("marks thinking block incomplete while tools are still running", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "", "checking"),
      tool("t1", "running"),
    ];

    const rows = buildTimelineRows(items, new Map());
    const block = rows.find((r) => r.kind === "thinking-block");
    expect(block?.kind).toBe("thinking-block");
    if (block?.kind !== "thinking-block") throw new Error("expected thinking block");
    expect(block.block.complete).toBe(false);
    expect(block.block.entries[0]?.kind).toBe("tool");
  });

  it("shows opening narration before thinking and folds later rounds", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "find training logs" },
      assistant("a1", "帮你找一下文件里的训练数据。", "round 1 thought"),
      tool("t1"),
      assistant("a2", "探索结果出来了，我去看关键文件。", "round 2 thought"),
      tool("t2"),
      assistant("a3", "找到了，我来读取。", "round 3 thought"),
      tool("t3"),
      assistant("a4", "训练用了 3 小时，样本 12000 条。", "round 4 thought"),
    ];

    const rows = buildTimelineRows(items, new Map());
    expect(rowKinds(rows)).toEqual(["user", "assistant", "thinking", "assistant"]);

    const preface = rows[1];
    expect(preface?.kind).toBe("single");
    if (preface?.kind !== "single" || preface.item.kind !== "assistant") throw new Error("expected preface");
    expect(preface.item.text).toBe("帮你找一下文件里的训练数据。");

    const block = rows[2];
    expect(block?.kind).toBe("thinking-block");
    if (block?.kind !== "thinking-block") throw new Error("expected thinking block");
    expect(block.block.entries).toHaveLength(3);
    expect(block.block.reasoning).toContain("round 1 thought");
    expect(block.block.reasoning).not.toContain("帮你找一下文件里的训练数据。");

    const answer = rows[3];
    expect(answer?.kind).toBe("single");
    if (answer?.kind !== "single" || answer.item.kind !== "assistant") throw new Error("expected assistant");
    expect(answer.item.text).toBe("训练用了 3 小时，样本 12000 条。");
  });

  it("shows background job completion notices immediately instead of batching at turn end", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "run tasks" },
      assistant("a1", "", "dispatching subagents"),
      tool("t1"),
      { kind: "notice", id: "n1", level: "info", text: "background task finished: task-1" },
      tool("t2"),
      { kind: "notice", id: "n2", level: "info", text: "background task finished: task-2" },
      assistant("a2", "both tasks finished"),
    ];

    const rows = buildTimelineRows(items, new Map());
    expect(rowKinds(rows)).toEqual(["user", "thinking", "notice", "thinking", "notice", "assistant"]);

    const firstNotice = rows[2];
    expect(firstNotice?.kind).toBe("single");
    if (firstNotice?.kind !== "single" || firstNotice.item.kind !== "notice") throw new Error("expected notice");
    expect(firstNotice.item.text).toContain("task-1");

    const secondNotice = rows[4];
    expect(secondNotice?.kind).toBe("single");
    if (secondNotice?.kind !== "single" || secondNotice.item.kind !== "notice") throw new Error("expected notice");
    expect(secondNotice.item.text).toContain("task-2");
  });

  it("drops duplicate preface and keeps thinking unified across truncation notices", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "analyze text flow" },
      assistant("a1", "好的，我来分析一下文本检测流程。", "thought 1"),
      tool("t1"),
      { kind: "notice", id: "n1", level: "info", text: "tool output truncated: 681 of 33449 bytes elided" },
      tool("t2"),
      assistant("a2", "好的，我来分析一下文本检测流程。", "thought 2"),
      assistant("a3", "好的，我来分析一下文本检测流程。入口在 common/config。", "thought 3"),
    ];

    const rows = buildTimelineRows(items, new Map());
    expect(rowKinds(rows)).toEqual(["user", "assistant", "thinking", "notice", "assistant"]);

    const thinking = rows[2];
    expect(thinking?.kind).toBe("thinking-block");
    if (thinking?.kind !== "thinking-block") throw new Error("expected thinking block");
    expect(thinking.block.entries).toHaveLength(2);

    const answer = rows[4];
    expect(answer?.kind).toBe("single");
    if (answer?.kind !== "single" || answer.item.kind !== "assistant") throw new Error("expected assistant");
    expect(answer.item.text).toBe("入口在 common/config。");
  });

  it("renders shell commands as standalone rows outside thinking blocks", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "", "checking repo"),
      tool("t1"),
      bashTool("b1", "cd desktop/frontend && npm run test:unit", "done", "Tests passed"),
      assistant("a2", "done"),
    ];

    const rows = buildTimelineRows(items, new Map());
    expect(rowKinds(rows)).toEqual(["user", "thinking", "tool", "assistant"]);

    const thinking = rows[1];
    expect(thinking?.kind).toBe("thinking-block");
    if (thinking?.kind !== "thinking-block") throw new Error("expected thinking block");
    expect(thinking.block.entries).toHaveLength(1);
    expect(thinking.block.entries[0]?.kind).toBe("tool");

    const shell = rows[2];
    expect(shell?.kind).toBe("single");
    if (shell?.kind !== "single" || shell.item.kind !== "tool") throw new Error("expected shell tool");
    expect(shell.item.name).toBe("bash");
  });
});
