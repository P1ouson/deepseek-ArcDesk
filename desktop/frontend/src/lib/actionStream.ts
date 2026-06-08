import { basename } from "./workspaceFilePreview";
import { subjectOf } from "./tools";
import type { Item, LiveStream, ToolStatus } from "./useController";

export type ToolItem = Extract<Item, { kind: "tool" }>;
export type AssistantItem = Extract<Item, { kind: "assistant" }>;

export interface ActionFileRef {
  fileName: string;
  relativePath: string;
  openPath: string;
}

export type SegmentEntry =
  | { kind: "tool"; item: ToolItem; subcalls?: ToolItem[] }
  | { kind: "thinking"; id: string; status: ToolStatus };

export interface ActionSegment {
  id: string;
  entries: SegmentEntry[];
  complete: boolean;
}

export type TimelineRow =
  | { kind: "single"; item: Item }
  | { kind: "action-segment"; segment: ActionSegment };

const WRITE_TOOLS = new Set(["edit_file", "write_file", "multi_edit"]);
const READ_TOOLS = new Set(["read_file", "grep", "glob", "ls"]);

function parseArgs(args: string): Record<string, unknown> {
  try {
    return JSON.parse(args) as Record<string, unknown>;
  } catch {
    return {};
  }
}

function argStr(args: Record<string, unknown>, key: string): string {
  return typeof args[key] === "string" ? (args[key] as string) : "";
}

function normalizeSlashes(path: string): string {
  return path.replace(/\\/g, "/");
}

export function toRelativePath(path: string, workspaceRoot: string): string {
  const clean = normalizeSlashes(path.trim());
  const root = normalizeSlashes(workspaceRoot.trim()).replace(/\/$/, "");
  if (!root) return clean;
  const lowerClean = clean.toLowerCase();
  const lowerRoot = root.toLowerCase();
  if (lowerClean === lowerRoot) return "";
  if (lowerClean.startsWith(`${lowerRoot}/`)) {
    return clean.slice(root.length + 1);
  }
  return clean.replace(/^\.\//, "");
}

function makeFileRef(path: string, workspaceRoot: string): ActionFileRef {
  const openPath = normalizeSlashes(path);
  const relativePath = toRelativePath(openPath, workspaceRoot);
  return {
    fileName: basename(openPath),
    relativePath,
    openPath,
  };
}

function uniqueFileRefs(paths: string[], workspaceRoot: string): ActionFileRef[] {
  const seen = new Set<string>();
  const out: ActionFileRef[] = [];
  for (const raw of paths) {
    const trimmed = raw.trim();
    if (!trimmed) continue;
    const key = normalizeSlashes(trimmed);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(makeFileRef(key, workspaceRoot));
  }
  return out;
}

function pathsFromGrepOutput(output: string): string[] {
  const paths: string[] = [];
  for (const line of output.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const match = trimmed.match(/^(.+?):(\d+)(?::|$)/);
    paths.push(match ? match[1]! : trimmed);
  }
  return paths;
}

function pathsFromListOutput(output: string): string[] {
  return output
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

export function filesForTool(item: ToolItem, workspaceRoot: string): ActionFileRef[] {
  const args = parseArgs(item.args);
  switch (item.name) {
    case "read_file":
    case "edit_file":
    case "write_file": {
      const path = argStr(args, "path") || argStr(args, "file_path");
      return path ? [makeFileRef(path, workspaceRoot)] : [];
    }
    case "multi_edit": {
      const edits = Array.isArray(args.edits) ? (args.edits as Record<string, unknown>[]) : [];
      return uniqueFileRefs(
        edits.map((step) => argStr(step, "path") || argStr(step, "file_path")).filter(Boolean),
        workspaceRoot,
      );
    }
    case "grep":
      return item.output ? uniqueFileRefs(pathsFromGrepOutput(item.output), workspaceRoot) : [];
    case "glob":
    case "ls":
      return item.output ? uniqueFileRefs(pathsFromListOutput(item.output), workspaceRoot) : [];
    default:
      return [];
  }
}

export function verbForTool(name: string, status: ToolStatus): string {
  const running = status === "running";
  switch (name) {
    case "grep":
    case "glob":
      return running ? "Grepping" : "Grepped";
    case "read_file":
      return running ? "Reading" : "Read";
    case "web_fetch":
      return running ? "Fetching" : "Fetched";
    case "edit_file":
    case "write_file":
    case "multi_edit":
      return running ? "Editing" : "Edited";
    case "bash":
      return running ? "Running" : "Ran";
    case "ls":
      return running ? "Listing" : "Listed";
    case "task":
    case "run_skill":
    case "explore":
    case "research":
    case "security_review":
      return running ? "Working" : "Worked";
    default:
      return running ? "Running" : "Ran";
  }
}

export function verbForThinking(status: ToolStatus): string {
  return status === "running" ? "Thinking" : "Thought";
}

export function isWriteTool(name: string): boolean {
  return WRITE_TOOLS.has(name);
}

export function isReadTool(name: string): boolean {
  return READ_TOOLS.has(name);
}

export function canOpenFileFromTool(item: ToolItem): boolean {
  if (item.status === "running") return false;
  if (isWriteTool(item.name)) return filesForTool(item, "").length > 0 || !!item.fileDiff?.diff;
  if (isReadTool(item.name) || item.name === "read_file") return filesForTool(item, "").length > 0;
  return filesForTool(item, "").length > 0;
}

export function subjectLabel(item: ToolItem): string {
  return subjectOf(item.name, item.args);
}

export function segmentIsRunning(segment: ActionSegment): boolean {
  return segment.entries.some((entry) => {
    if (entry.kind === "thinking") return entry.status === "running";
    return entry.item.status === "running";
  });
}

const SKIPPED_TOOLS = new Set(["todo_write", "exit_plan_mode"]);

function isRootTool(item: Item): item is ToolItem {
  return item.kind === "tool" && !item.parentId && !SKIPPED_TOOLS.has(item.name);
}

/** Kernel creates the assistant bubble before tool rows; reorder to tools → assistant per turn. */
function normalizeTurnOrder(items: Item[]): Item[] {
  const out: Item[] = [];
  let bufTools: ToolItem[] = [];
  let bufAssistant: AssistantItem | null = null;

  const flushBuffer = () => {
    for (const tool of bufTools) out.push(tool);
    if (bufAssistant) out.push(bufAssistant);
    bufTools = [];
    bufAssistant = null;
  };

  for (const item of items) {
    if (item.kind === "user") {
      flushBuffer();
      out.push(item);
      continue;
    }
    if (isRootTool(item)) {
      bufTools.push(item);
      continue;
    }
    if (item.kind === "assistant") {
      bufAssistant = item;
      continue;
    }
    flushBuffer();
    out.push(item);
  }
  flushBuffer();
  return out;
}

export function buildTimelineRows(
  items: Item[],
  subcallsByParent: Map<string, ToolItem[]>,
  live?: LiveStream,
): TimelineRow[] {
  const rows: TimelineRow[] = [];
  let segmentEntries: SegmentEntry[] = [];
  let segmentId = "";

  const flushSegment = (complete: boolean) => {
    if (segmentEntries.length === 0) return;
    rows.push({
      kind: "action-segment",
      segment: {
        id: segmentId || `seg-${rows.length}`,
        entries: segmentEntries,
        complete,
      },
    });
    segmentEntries = [];
    segmentId = "";
  };

  const pushTool = (item: ToolItem) => {
    if (segmentEntries.length === 0) segmentId = `seg-${item.id}`;
    segmentEntries.push({
      kind: "tool",
      item,
      subcalls: subcallsByParent.get(item.id),
    });
  };

  for (const item of normalizeTurnOrder(items)) {
    if (item.kind === "tool") {
      pushTool(item);
      continue;
    }

    if (item.kind === "assistant") {
      const isLive = live?.id === item.id;
      const reasoning = (isLive ? live.reasoning : item.reasoning)?.trim() ?? "";
      const text = (isLive ? live.text : item.text)?.trim() ?? "";
      const streaming = item.streaming || (isLive && !text);

      if (reasoning) {
        if (segmentEntries.length === 0) segmentId = `seg-think-${item.id}`;
        const thinkingIdx = segmentEntries.findIndex((e) => e.kind === "thinking" && e.id === item.id);
        const thinkingEntry: SegmentEntry = {
          kind: "thinking",
          id: item.id,
          status: streaming && !text ? "running" : "done",
        };
        if (thinkingIdx >= 0) {
          segmentEntries[thinkingIdx] = thinkingEntry;
        } else if (segmentEntries.length > 0) {
          segmentEntries.unshift(thinkingEntry);
        } else {
          segmentEntries.push(thinkingEntry);
        }
      }

      if (text) {
        flushSegment(true);
        rows.push({
          kind: "single",
          item: { ...item, reasoning: "", text: isLive ? live!.text : item.text, streaming: item.streaming },
        });
        continue;
      }

      if (reasoning || streaming) continue;
    }

    flushSegment(true);
    rows.push({ kind: "single", item });
  }

  if (segmentEntries.length > 0) {
    const complete = !segmentIsRunning({ id: segmentId, entries: segmentEntries, complete: false });
    flushSegment(complete);
  }

  return rows;
}
