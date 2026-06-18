import { buildTimelineRows, type ToolItem } from "./actionStream";
import type { Item, LiveStream } from "./useController";

function assistantText(item: Extract<Item, { kind: "assistant" }>, live?: LiveStream): string {
  if (live?.id === item.id) return live.text.trim();
  return item.text.trim();
}

function isStreamingAssistant(item: Extract<Item, { kind: "assistant" }>, live?: LiveStream): boolean {
  return item.streaming || live?.id === item.id;
}

/** Copy + rewind/continue actions attach to the last visible assistant bubble in each turn. */
export function deriveTurnAssistantActions(
  items: Item[],
  subcallsByParent: Map<string, ToolItem[]>,
  live?: LiveStream,
  turnActive = false,
  rows = buildTimelineRows(items, subcallsByParent, live, turnActive),
): {
  copyTextByAssistantId: Map<string, string>;
  showActionsByAssistantId: Map<string, boolean>;
} {
  const copyTextByAssistantId = new Map<string, string>();
  const showActionsByAssistantId = new Map<string, boolean>();

  const turnGroups: typeof rows[] = [];
  let group: typeof rows = [];
  for (const row of rows) {
    if (row.kind === "single" && row.item.kind === "user") {
      if (group.length > 0) turnGroups.push(group);
      group = [row];
      continue;
    }
    group.push(row);
  }
  if (group.length > 0) turnGroups.push(group);

  const itemGroups: Item[][] = [];
  let itemGroup: Item[] = [];
  for (const it of items) {
    if (it.kind === "user") {
      if (itemGroup.length > 0) itemGroups.push(itemGroup);
      itemGroup = [it];
      continue;
    }
    itemGroup.push(it);
  }
  if (itemGroup.length > 0) itemGroups.push(itemGroup);

  for (let i = 0; i < turnGroups.length; i++) {
    const rowGroup = turnGroups[i]!;
    const itemsInTurn = itemGroups[i] ?? [];

    const chunks: string[] = [];
    for (const it of itemsInTurn) {
      if (it.kind !== "assistant") continue;
      const text = assistantText(it, live);
      if (text) chunks.push(live?.id === it.id ? live.text : it.text);
    }
    if (chunks.length === 0) continue;

    const copyText = chunks.join("\n\n");
    for (let r = rowGroup.length - 1; r >= 0; r--) {
      const row = rowGroup[r]!;
      if (row.kind !== "single" || row.item.kind !== "assistant") continue;
      const item = row.item;
      if (isStreamingAssistant(item, live)) continue;
      if (!assistantText(item, live)) continue;
      copyTextByAssistantId.set(item.id, copyText);
      showActionsByAssistantId.set(item.id, true);
      break;
    }
  }

  return { copyTextByAssistantId, showActionsByAssistantId };
}
