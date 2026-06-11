import type { Translator } from "./i18n";
import { parseToolArgs, toolArgString } from "./parseToolArgs";
import type { Item } from "./useController";

export type TurnPhase = "scanning" | "reading" | "analyzing" | "planning" | "segmented" | "idle";

export interface TurnProgress {
  phase: TurnPhase;
  label: string;
  detail?: string;
  elapsedMs: number;
  showSlowHint: boolean;
  showProgressDetail: boolean;
}

const SCAN_TOOLS = new Set(["grep", "glob", "ls", "task", "explore", "research"]);
const READ_TOOLS = new Set(["read_file", "web_fetch"]);

function basename(path: string): string {
  const normalized = path.replace(/\\/g, "/");
  const parts = normalized.split("/").filter(Boolean);
  return parts[parts.length - 1] ?? path;
}

function toolPath(item: Extract<Item, { kind: "tool" }>): string {
  const args = parseToolArgs(item.args);
  return (
    toolArgString(args, "path") ||
    toolArgString(args, "file_path") ||
    toolArgString(args, "pattern") ||
    toolArgString(args, "url") ||
    ""
  );
}

function hasRecentTruncationNotice(items: Item[]): boolean {
  for (let i = items.length - 1; i >= 0 && i >= items.length - 6; i--) {
    const it = items[i]!;
    if (it.kind === "notice" && /truncated|elided|分段/i.test(it.text)) return true;
  }
  return false;
}

function runningTruncatedRead(items: Item[]): Extract<Item, { kind: "tool" }> | undefined {
  return items.find(
    (it): it is Extract<Item, { kind: "tool" }> =>
      it.kind === "tool" && it.status === "running" && READ_TOOLS.has(it.name) && !!it.truncated,
  );
}

/** Maps live agent/tool state to a short human-readable progress line. */
export function deriveTurnProgress(input: {
  running: boolean;
  turnStartAt: number;
  items: Item[];
  now?: number;
  t: Translator;
}): TurnProgress | null {
  const { running, turnStartAt, items, t } = input;
  if (!running || turnStartAt <= 0) return null;

  const now = input.now ?? Date.now();
  const elapsedMs = Math.max(0, now - turnStartAt);
  const showSlowHint = elapsedMs >= 20_000;
  const showProgressDetail = elapsedMs >= 3_000;

  const tools = items.filter((it): it is Extract<Item, { kind: "tool" }> => it.kind === "tool");
  const runningTools = tools.filter((it) => it.status === "running");
  const readTools = tools.filter((it) => READ_TOOLS.has(it.name));
  const completedReads = readTools.filter((it) => it.status === "done" || it.status === "error").length;
  const runningReads = readTools.filter((it) => it.status === "running");

  if (hasRecentTruncationNotice(items) || runningTruncatedRead(items)) {
    return {
      phase: "segmented",
      label: t("turnProgress.segmented"),
      detail: showProgressDetail ? t("turnProgress.segmentedDetail") : undefined,
      elapsedMs,
      showSlowHint,
      showProgressDetail,
    };
  }

  if (runningReads.length > 0) {
    const current = runningReads[runningReads.length - 1]!;
    const path = toolPath(current);
    const detail =
      showProgressDetail
        ? t("turnProgress.readingDetail", {
            done: completedReads,
            total: completedReads + runningReads.length,
            file: path ? basename(path) : t("turnProgress.filesGeneric"),
          })
        : undefined;
    return {
      phase: "reading",
      label: t("turnProgress.reading"),
      detail,
      elapsedMs,
      showSlowHint,
      showProgressDetail,
    };
  }

  const scanning = runningTools.filter((it) => SCAN_TOOLS.has(it.name));
  if (scanning.length > 0) {
    const current = scanning[scanning.length - 1]!;
    const subject = toolPath(current);
    const detail =
      showProgressDetail && subject
        ? t("turnProgress.scanningDetail", { target: basename(subject) || subject })
        : showProgressDetail
          ? t("turnProgress.scanningDetailGeneric")
          : undefined;
    return {
      phase: "scanning",
      label: t("turnProgress.scanning"),
      detail,
      elapsedMs,
      showSlowHint,
      showProgressDetail,
    };
  }

  if (runningTools.some((it) => it.name === "write_file" || it.name === "edit_file" || it.name === "multi_edit")) {
    return {
      phase: "planning",
      label: t("turnProgress.planning"),
      elapsedMs,
      showSlowHint,
      showProgressDetail,
    };
  }

  if (runningTools.length > 0 || tools.some((it) => it.status === "running")) {
    return {
      phase: "analyzing",
      label: t("turnProgress.analyzing"),
      detail: showProgressDetail ? t("turnProgress.analyzingDetail") : undefined,
      elapsedMs,
      showSlowHint,
      showProgressDetail,
    };
  }

  return {
    phase: "analyzing",
    label: t("turnProgress.thinking"),
    detail: showProgressDetail ? t("turnProgress.thinkingDetail") : undefined,
    elapsedMs,
    showSlowHint,
    showProgressDetail,
  };
}
