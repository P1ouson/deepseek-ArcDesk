import { describe, expect, it } from "vitest";
import {
  deriveThinkingBlockHint,
  deriveThinkingBlockTitle,
  summarizeThinkingBlockStats,
  verbLabelForTool,
} from "./thinkingBlockLabels";
import type { ThinkingBlock, ToolItem } from "./actionStream";

const t = (key: string, vars?: Record<string, string | number>) => {
  if (key === "thinkingBlock.title.exploring") return "探索代码库";
  if (key === "thinkingBlock.title.reading") return "阅读关键文件";
  if (key === "thinkingBlock.title.planning") return "规划下一步";
  if (key === "thinkingBlock.hint.searching") return `搜索 ${vars?.target ?? ""}`;
  if (key === "thinkingBlock.stat.reads") return `读取 ${vars?.n} 个文件`;
  if (key === "thinkingBlock.stat.searches") return `搜索 ${vars?.n} 次`;
  if (key === "thinkingBlock.verb.grep") return "搜索代码";
  if (key === "thinkingBlock.verbDone.readFile") return "已读取";
  return key;
};

function tool(name: string, status: ToolItem["status"] = "done", args = "{}"): ToolItem {
  return { kind: "tool", id: name, name, args, readOnly: true, status };
}

function block(entries: ThinkingBlock["entries"], reasoning = "", streaming = false): ThinkingBlock {
  return {
    id: "b1",
    reasoning,
    entries,
    streaming,
    complete: !streaming,
    turnInProgress: streaming,
  };
}

describe("thinkingBlockLabels", () => {
  it("titles explore-heavy blocks", () => {
    const title = deriveThinkingBlockTitle(
      block([{ kind: "tool", item: tool("grep", "running", '{"pattern":"TODO"}') }]),
      t,
    );
    expect(title).toBe("探索代码库");
  });

  it("titles read-heavy blocks", () => {
    const title = deriveThinkingBlockTitle(
      block([
        { kind: "tool", item: tool("read_file", "done", '{"path":"src/a.go"}') },
        { kind: "tool", item: tool("read_file", "running", '{"path":"src/b.go"}') },
      ]),
      t,
    );
    expect(title).toBe("阅读关键文件");
  });

  it("summarizes completed tool mix", () => {
    const summary = summarizeThinkingBlockStats(
      [
        tool("read_file"),
        tool("read_file"),
        tool("grep"),
        tool("grep"),
        tool("grep"),
      ],
      t,
    );
    expect(summary).toBe("读取 2 个文件 · 搜索 3 次");
  });

  it("shows active search hint", () => {
    const hint = deriveThinkingBlockHint(
      block([{ kind: "tool", item: tool("grep", "running", '{"pattern":"cache","path":"internal"}') }]),
      t,
    );
    expect(hint).toBe("搜索 cache");
  });

  it("localizes tool verbs", () => {
    expect(verbLabelForTool("grep", "running", t)).toBe("搜索代码");
    expect(verbLabelForTool("read_file", "done", t)).toBe("已读取");
  });
});
