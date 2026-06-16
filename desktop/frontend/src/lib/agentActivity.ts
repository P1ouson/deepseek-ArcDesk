import { isContinueTextStream } from "./continueGeneration";
import { parseToolArgs, toolArgString } from "./parseToolArgs";
import type { Item, State } from "./useController";

const DEV_SERVER_CMD =
  /\b(pnpm\s+(run\s+)?dev|npm\s+run\s+dev|yarn\s+dev|bun\s+run\s+dev|vite(\s|$)|next\s+dev|npx\s+vite|webpack\s+serve|ng\s+serve)\b/i;

const LOCALHOST_URL = /https?:\/\/(?:localhost|127\.0\.0\.1|\[::1\]|::1)(?::\d+)?(?:\/[^\s"'`]*)?/i;

export function isDevServerBashTool(item: Extract<Item, { kind: "tool" }>): boolean {
  if (item.name !== "bash") return false;
  const args = parseToolArgs(item.args);
  const cmd = toolArgString(args, "command") || item.args;
  return DEV_SERVER_CMD.test(cmd);
}

export function extractLocalhostUrl(text: string): string | null {
  const match = text.match(LOCALHOST_URL);
  return match?.[0] ?? null;
}

/** Agent work the user can cancel (excludes long-running dev-server bash). */
export function isCancellableAgentWork(
  state: Pick<
    State,
    "continueActive" | "running" | "turnActive" | "live" | "pendingUser" | "approval" | "ask" | "items"
  >,
): boolean {
  if (isContinueTextStream(state)) return false;
  if (!state.running) return false;
  if (state.live || state.pendingUser || state.approval || state.ask) return true;
  const runningTools = state.items.filter(
    (it): it is Extract<Item, { kind: "tool" }> => it.kind === "tool" && it.status === "running",
  );
  if (runningTools.length === 0) return state.turnActive;
  return runningTools.some((tool) => !isDevServerBashTool(tool));
}

/** Block concurrent send only when non-dev-server agent work is active. */
export function shouldBlockAgentSend(
  state: Pick<
    State,
    "continueActive" | "running" | "turnActive" | "live" | "pendingUser" | "approval" | "ask" | "items"
  >,
): boolean {
  if (isContinueTextStream(state)) return false;
  if (!state.running && !state.turnActive) return false;
  if (state.pendingUser || state.approval || state.ask || state.live) return true;
  const runningTools = state.items.filter(
    (it): it is Extract<Item, { kind: "tool" }> => it.kind === "tool" && it.status === "running",
  );
  if (runningTools.some((tool) => !isDevServerBashTool(tool))) return true;
  if (runningTools.length > 0) return false;
  return state.running && state.turnActive;
}

export function findDevPreviewTrigger(items: Item[]): { url?: string; openPanel: boolean } | null {
  for (let i = items.length - 1; i >= 0; i--) {
    const item = items[i]!;
    if (item.kind !== "tool" || item.name !== "bash") continue;
    const blob = [item.output, item.error, item.args].filter(Boolean).join("\n");
    const url = extractLocalhostUrl(blob);
    if (url) return { url, openPanel: true };
    if (isDevServerBashTool(item) && (item.status === "running" || item.status === "done")) {
      return { openPanel: true };
    }
  }
  return null;
}
