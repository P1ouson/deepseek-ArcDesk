import type { Item } from "./useController";

/** Backend notice when the model hits max output tokens or repetition guard. */
export function isResponseTruncationNotice(text: string): boolean {
  const normalized = text.trim().toLowerCase();
  return (
    normalized.startsWith("response truncated:") ||
    normalized.includes("response truncated: hit max output tokens") ||
    normalized.includes("response truncated: model repetition detected")
  );
}

/** Map assistant message ids that were cut off in their turn. */
export function truncatedAssistantIds(items: Item[]): Set<string> {
  const out = new Set<string>();
  let lastAssistantId: string | undefined;

  for (const it of items) {
    if (it.kind === "user") {
      lastAssistantId = undefined;
      continue;
    }
    if (it.kind === "assistant" && it.text.trim()) {
      lastAssistantId = it.id;
      continue;
    }
    if (it.kind === "notice" && isResponseTruncationNotice(it.text) && lastAssistantId) {
      out.add(lastAssistantId);
    }
  }

  return out;
}

export function assistantSeedText(items: Item[], assistantId: string): { text: string; reasoning: string } {
  const item = items.find((it) => it.kind === "assistant" && it.id === assistantId);
  if (!item || item.kind !== "assistant") return { text: "", reasoning: "" };
  return { text: item.text, reasoning: item.reasoning };
}
