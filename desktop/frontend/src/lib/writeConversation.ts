import type { Item, LiveStream } from "./useController";

export interface WriteTurn {
  id: string;
  role: "user" | "assistant";
  text: string;
  reasoning?: string;
  streaming?: boolean;
}

export function buildWriteConversation(items: Item[], live?: LiveStream): WriteTurn[] {
  const turns: WriteTurn[] = [];
  for (const item of items) {
    if (item.kind === "user") {
      const text = item.text.trim();
      if (!text) continue;
      turns.push({ id: item.id, role: "user", text: item.text.trim() });
      continue;
    }
    if (item.kind === "assistant") {
      if (!item.text.trim() && !item.streaming) continue;
      turns.push({
        id: item.id,
        role: "assistant",
        text: item.text,
        reasoning: item.reasoning,
        streaming: item.streaming,
      });
    }
  }

  if (!live?.text.trim() && !live?.reasoning?.trim()) return turns;

  const last = turns[turns.length - 1];
  if (last?.role === "assistant" && last.streaming) {
    turns[turns.length - 1] = {
      ...last,
      text: live.text,
      reasoning: live.reasoning,
      streaming: true,
    };
    return turns;
  }

  turns.push({
    id: live?.id ?? "live",
    role: "assistant",
    text: live?.text ?? "",
    reasoning: live?.reasoning,
    streaming: true,
  });
  return turns;
}

export function latestWriteAssistant(turns: WriteTurn[]): WriteTurn | null {
  for (let i = turns.length - 1; i >= 0; i--) {
    const turn = turns[i];
    if (turn?.role === "assistant" && turn.text.trim()) return turn;
  }
  return null;
}
