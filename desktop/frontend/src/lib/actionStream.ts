import { basename } from "./workspaceFilePreview";
import { parseToolArgs, toolArgString } from "./parseToolArgs";
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
  const args = parseToolArgs(item.args);
  switch (item.name) {
    case "read_file":
    case "edit_file":
    case "write_file": {
      const path = toolArgString(args, "path") || toolArgString(args, "file_path");
      return path ? [makeFileRef(path, workspaceRoot)] : [];
    }
    case "multi_edit": {
      const edits = Array.isArray(args.edits) ? (args.edits as Record<string, unknown>[]) : [];
      return uniqueFileRefs(
        edits.map((step) => toolArgString(step, "path") || toolArgString(step, "file_path")).filter(Boolean),
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

/** Interleave assistant narration and tool rows in timeline order (Cursor-style). */
export function buildTimelineRows(
  items: Item[],
  subcallsByParent: Map<string, ToolItem[]>,
  live?: LiveStream,
): TimelineRow[] {
  const rows: TimelineRow[] = [];
  let toolBuffer: ToolItem[] = [];

  const flushTools = () => {
    if (toolBuffer.length === 0) return;
    const entries: SegmentEntry[] = toolBuffer.map((item) => ({
      kind: "tool",
      item,
      subcalls: subcallsByParent.get(item.id),
    }));
    const complete = !entries.some((entry) => entry.kind === "tool" && entry.item.status === "running");
    rows.push({
      kind: "action-segment",
      segment: {
        id: `seg-${toolBuffer[0]!.id}`,
        entries,
        complete,
      },
    });
    toolBuffer = [];
  };

  for (const item of items) {
    if (isRootTool(item)) {
      toolBuffer.push(item);
      continue;
    }

    flushTools();

    if (item.kind === "assistant") {
      const isLive = live?.id === item.id;
      const text = isLive ? live.text : item.text;
      const reasoning = isLive ? live.reasoning : item.reasoning;
      const hasText = text.trim().length > 0;
      const hasReasoning = reasoning.trim().length > 0;
      const streaming = item.streaming || isLive === true;

      if (hasText || hasReasoning || streaming) {
        rows.push({
          kind: "single",
          item: {
            ...item,
            text,
            reasoning,
            streaming: item.streaming || isLive === true,
          },
        });
      }
      continue;
    }

    rows.push({ kind: "single", item });
  }

  flushTools();
  return coalesceReasoningAcrossTools(rows, live);
}

/** Merge reasoning-only assistant rows in a turn into one block (tools stay in between). */
function coalesceReasoningAcrossTools(rows: TimelineRow[], live?: LiveStream): TimelineRow[] {
  const result: TimelineRow[] = [];
  const reasoningParts: string[] = [];
  let reasoningStreaming = false;
  let anchorId = "";
  let insertAt = -1;
  let mergedInserted = false;
  const absorbedIds: string[] = [];

  const resetReasoning = () => {
    reasoningParts.length = 0;
    reasoningStreaming = false;
    anchorId = "";
    insertAt = -1;
    mergedInserted = false;
    absorbedIds.length = 0;
  };

  const insertMergedReasoning = () => {
    if (reasoningParts.length === 0 || mergedInserted) return;
    const mergedId = live && absorbedIds.includes(live.id) ? live.id : anchorId || "merged-reasoning";
    const mergedRow: TimelineRow = {
      kind: "single",
      item: {
        kind: "assistant",
        id: mergedId,
        text: "",
        reasoning: reasoningParts.join("\n\n"),
        streaming: reasoningStreaming,
      },
    };
    if (insertAt >= 0 && insertAt <= result.length) {
      result.splice(insertAt, 0, mergedRow);
    } else {
      result.push(mergedRow);
    }
    mergedInserted = true;
    reasoningParts.length = 0;
    reasoningStreaming = false;
    anchorId = "";
    insertAt = -1;
    absorbedIds.length = 0;
  };

  for (const row of rows) {
    if (row.kind === "single" && row.item.kind === "user") {
      insertMergedReasoning();
      resetReasoning();
      result.push(row);
      continue;
    }

    if (row.kind === "single" && row.item.kind === "assistant") {
      const a = row.item;
      const isLive = live?.id === a.id;
      const hasText = a.text.trim().length > 0;
      const hasReasoning = a.reasoning.trim().length > 0;

      if (!hasText && hasReasoning) {
        if (reasoningParts.length === 0) {
          insertAt = result.length;
          anchorId = isLive && live ? live.id : a.id;
        } else if (isLive && live) {
          anchorId = live.id;
        }
        absorbedIds.push(a.id);
        reasoningParts.push(a.reasoning);
        reasoningStreaming = reasoningStreaming || a.streaming || isLive;
        continue;
      }

      if (hasText) {
        if (hasReasoning) {
          if (reasoningParts.length === 0) {
            insertAt = result.length;
            anchorId = isLive && live ? live.id : a.id;
          } else if (isLive && live) {
            anchorId = live.id;
          }
          absorbedIds.push(a.id);
          reasoningParts.push(a.reasoning);
          reasoningStreaming = reasoningStreaming || a.streaming || isLive;
        }
        insertMergedReasoning();
        result.push({ kind: "single", item: { ...a, reasoning: "" } });
        continue;
      }

      if (a.streaming) {
        insertMergedReasoning();
        result.push(row);
        continue;
      }

      continue;
    }

    result.push(row);
  }

  insertMergedReasoning();
  return result;
}
