import { describe, expect, it } from "vitest";
import type { Item } from "./useController";
import { buildTimelineRows } from "./actionStream";
import { deriveTurnAssistantActions } from "./turnAssistantActions";

function assistant(id: string, text = "", reasoning = "", streaming = false): Item {
  return { kind: "assistant", id, text, reasoning, streaming };
}

describe("deriveTurnAssistantActions", () => {
  it("targets the last visible assistant bubble after thinking blocks", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "fix it" },
      assistant("a1", "帮你找一下文件里的训练数据。"),
      assistant("a2", "", "thought"),
      assistant("a3", "训练用了 3 小时，样本 12000 条。"),
    ];
    const subcalls = new Map();
    const rows = buildTimelineRows(items, subcalls);
    const answerRow = rows.find((row) => row.kind === "single" && row.item.kind === "assistant" && row.item.id === "a3");
    expect(answerRow).toBeTruthy();

    const actions = deriveTurnAssistantActions(items, subcalls);
    expect(actions.showActionsByAssistantId.get("a3")).toBe(true);
    expect(actions.showActionsByAssistantId.get("a1")).toBeUndefined();
    expect(actions.copyTextByAssistantId.get("a3")).toContain("训练用了 3 小时");
    expect(actions.copyTextByAssistantId.get("a3")).toContain("帮你找一下文件里的训练数据。");
  });

  it("does not attach actions to streaming assistant rows", () => {
    const items: Item[] = [
      { kind: "user", id: "u1", text: "hi" },
      assistant("a1", "partial", "", true),
    ];
    const subcalls = new Map();
    const actions = deriveTurnAssistantActions(items, subcalls, { id: "a1", text: "partial live", reasoning: "" });
    expect(actions.showActionsByAssistantId.get("a1")).toBeUndefined();
  });
});
