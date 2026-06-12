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

export interface ThinkingBlock {
  id: string;
  reasoning: string;
  entries: SegmentEntry[];
  streaming: boolean;
  complete: boolean;
}

export type TimelineRow =
  | { kind: "single"; item: Item }
  | { kind: "thinking-block"; block: ThinkingBlock }
  | { kind: "action-segment"; segment: ActionSegment };

const WRITE_TOOLS = new Set(["edit_file", "write_file", "multi_edit"]);

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

export function subjectLabel(item: ToolItem): string {
  return subjectOf(item.name, item.args);
}

export function thinkingBlockIsActive(block: ThinkingBlock): boolean {
  if (block.streaming) return true;
  if (!block.complete) return true;
  return block.entries.some((entry) => entry.kind === "tool" && entry.item.status === "running");
}

/** Shell/bash runs get their own timeline card instead of living inside 思考过程. */
export function isShellTimelineTool(item: ToolItem): boolean {
  return item.name === "bash" || item.isShell === true;
}

const SKIPPED_TOOLS = new Set(["todo_write", "exit_plan_mode"]);

function isRootTool(item: Item): item is ToolItem {
  return item.kind === "tool" && !item.parentId && !SKIPPED_TOOLS.has(item.name);
}

function toolEntries(tools: ToolItem[], subcallsByParent: Map<string, ToolItem[]>): SegmentEntry[] {
  return tools.map((item) => ({
    kind: "tool" as const,
    item,
    subcalls: subcallsByParent.get(item.id),
  }));
}

function entriesAreComplete(entries: SegmentEntry[]): boolean {
  return !entries.some((entry) => entry.kind === "tool" && entry.item.status === "running");
}

type ThinkingDraft = {
  reasoningParts: string[];
  tools: ToolItem[];
  streaming: boolean;
  anchorId: string;
  absorbedIds: string[];
};

/** True when more assistant/tool work follows before the next user turn. */
function hasMoreAgentWork(items: Item[], fromIndex: number): boolean {
  for (let i = fromIndex + 1; i < items.length; i++) {
    const it = items[i]!;
    if (it.kind === "user") return false;
    if (it.kind === "tool" && isRootTool(it)) return true;
    if (it.kind === "assistant") return true;
  }
  return false;
}

/** Mid-turn narration (reasoning + short status text) should fold, not split per API round. */
function isInterimAssistant(items: Item[], index: number, live?: LiveStream): boolean {
  const item = items[index];
  if (item.kind !== "assistant") return false;
  if (live?.id === item.id) return true;
  return hasMoreAgentWork(items, index);
}

/** Remove a turn's opening line when the model repeats it on the final answer. */
function stripTurnPreface(preamble: string | null, text: string): string {
  if (!preamble) return text;
  const body = text.trim();
  const preface = preamble.trim();
  if (!body || !preface) return text;
  if (body === preface) return "";
  if (!body.startsWith(preface)) return text;
  const rest = body.slice(preface.length).replace(/^[\s，,。:：\-–—]+/, "").trim();
  return rest || text;
}

/** Background job lifecycle notices should appear when the job finishes, not after the whole turn. */
function isBackgroundJobNotice(item: Item): boolean {
  if (item.kind !== "notice") return false;
  return /background\s+(?:task|bash)\s+(?:started|finished|failed|killed)\b/i.test(item.text);
}

/** Interleave assistant narration and fold tool work into collapsible thinking blocks. */
export function buildTimelineRows(
  items: Item[],
  subcallsByParent: Map<string, ToolItem[]>,
  live?: LiveStream,
): TimelineRow[] {
  const rows: TimelineRow[] = [];
  let thinking: ThinkingDraft | null = null;
  let turnPreface: string | null = null;
  const deferredNotices: Item[] = [];

  const resetThinking = () => {
    thinking = null;
  };

  const ensureThinking = (): ThinkingDraft => {
    if (!thinking) {
      thinking = { reasoningParts: [], tools: [], streaming: false, anchorId: "", absorbedIds: [] };
    }
    return thinking;
  };

  const appendReasoning = (assistant: AssistantItem, isLive: boolean) => {
    const trimmed = assistant.reasoning.trim();
    if (!trimmed) return;
    const draft = ensureThinking();
    if (!draft.anchorId) draft.anchorId = isLive && live ? live.id : assistant.id;
    else if (isLive && live) draft.anchorId = live.id;
    if (!draft.absorbedIds.includes(assistant.id)) draft.absorbedIds.push(assistant.id);
    draft.reasoningParts.push(isLive && live ? live.reasoning : assistant.reasoning);
    draft.streaming = draft.streaming || assistant.streaming || isLive;
  };

  const flushThinking = () => {
    if (!thinking) return;
    const draft = thinking;
    const reasoning = draft.reasoningParts
      .map((part) => part.trim())
      .filter(Boolean)
      .join("\n\n");
    const entries = toolEntries(draft.tools, subcallsByParent);
    if (!reasoning && entries.length === 0) {
      resetThinking();
      return;
    }

    const liveAttached = live != null && draft.absorbedIds.includes(live.id);
    const liveReasoning = liveAttached ? live.reasoning.trim() : "";
    const mergedReasoning =
      liveReasoning && !draft.reasoningParts.some((part) => part.trim() === liveReasoning)
        ? [reasoning, liveReasoning].filter(Boolean).join("\n\n")
        : reasoning;

    rows.push({
      kind: "thinking-block",
      block: {
        id: liveAttached ? live.id : draft.anchorId || `think-${draft.tools[0]?.id ?? "reasoning"}`,
        reasoning: mergedReasoning,
        entries,
        streaming: draft.streaming || liveAttached,
        complete: !draft.streaming && !liveAttached && entriesAreComplete(entries),
      },
    });
    for (const notice of deferredNotices) {
      rows.push({ kind: "single", item: notice });
    }
    deferredNotices.length = 0;
    resetThinking();
  };

  for (let index = 0; index < items.length; index++) {
    const item = items[index]!;
    if (item.kind === "user") {
      flushThinking();
      turnPreface = null;
      rows.push({ kind: "single", item });
      continue;
    }

    if (item.kind === "notice" || item.kind === "phase") {
      if (item.kind === "notice" && isBackgroundJobNotice(item)) {
        flushThinking();
        rows.push({ kind: "single", item });
        continue;
      }
      if (thinking) {
        deferredNotices.push(item);
      } else {
        rows.push({ kind: "single", item });
      }
      continue;
    }

    if (isRootTool(item)) {
      if (isShellTimelineTool(item)) {
        flushThinking();
        rows.push({ kind: "single", item });
        continue;
      }
      const draft = ensureThinking();
      if (!draft.anchorId) draft.anchorId = `think-${item.id}`;
      draft.tools.push(item);
      continue;
    }

    if (item.kind === "assistant") {
      const isLive = live?.id === item.id;
      const text = isLive ? live.text : item.text;
      const reasoning = isLive ? live.reasoning : item.reasoning;
      const hasText = text.trim().length > 0;
      const hasReasoning = reasoning.trim().length > 0;
      const streaming = item.streaming || isLive === true;
      const interim = isInterimAssistant(items, index, live);

      if (!hasText && hasReasoning) {
        appendReasoning({ ...item, reasoning, streaming }, isLive);
        continue;
      }

      if (interim && hasText) {
        if (hasReasoning) {
          appendReasoning({ ...item, reasoning, streaming }, isLive);
        }
        const trimmed = text.trim();
        if (trimmed && !turnPreface) {
          turnPreface = trimmed;
          rows.push({
            kind: "single",
            item: {
              ...item,
              text,
              reasoning: "",
              streaming,
            },
          });
        }
        continue;
      }

      if (hasText) {
        if (hasReasoning) {
          appendReasoning({ ...item, reasoning, streaming }, isLive);
        }
        flushThinking();
        const finalText = stripTurnPreface(turnPreface, text);
        if (!finalText.trim()) continue;
        rows.push({
          kind: "single",
          item: {
            ...item,
            text: finalText,
            reasoning: "",
            streaming,
          },
        });
        continue;
      }

      if (streaming) {
        flushThinking();
        rows.push({
          kind: "single",
          item: {
            ...item,
            text,
            reasoning: "",
            streaming,
          },
        });
      }
      continue;
    }

    flushThinking();
    rows.push({ kind: "single", item });
  }

  flushThinking();
  return rows;
}
