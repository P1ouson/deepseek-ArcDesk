import type { DictKey } from "../locales/en";
import type { Translator } from "./i18n";
import { basename } from "./workspaceFilePreview";
import { parseToolArgs, toolArgString } from "./parseToolArgs";
import { subjectOf } from "./tools";
import type { SegmentEntry, ThinkingBlock, ToolItem } from "./actionStream";
import { thinkingBlockIsActive } from "./actionStream";
import type { ToolStatus } from "./useController";

export type ThinkingPhase =
  | "planning"
  | "exploring"
  | "reading"
  | "editing"
  | "verifying"
  | "delegating"
  | "working";

const WRITE_TOOLS = new Set(["edit_file", "write_file", "multi_edit"]);
const SEARCH_TOOLS = new Set(["grep", "glob", "ls"]);
const READ_TOOLS = new Set(["read_file", "web_fetch"]);
const DELEGATE_TOOLS = new Set(["task", "run_skill", "explore", "research", "security_review"]);

export interface ThinkingBlockCounts {
  reads: number;
  searches: number;
  edits: number;
  commands: number;
  delegates: number;
}

function collectToolItems(entries: SegmentEntry[]): ToolItem[] {
  const out: ToolItem[] = [];
  for (const entry of entries) {
    if (entry.kind !== "tool") continue;
    out.push(entry.item);
    for (const sub of entry.subcalls ?? []) out.push(sub);
  }
  return out;
}

function countByKind(tools: ToolItem[]): ThinkingBlockCounts {
  const counts: ThinkingBlockCounts = {
    reads: 0,
    searches: 0,
    edits: 0,
    commands: 0,
    delegates: 0,
  };
  for (const tool of tools) {
    if (READ_TOOLS.has(tool.name)) counts.reads++;
    else if (SEARCH_TOOLS.has(tool.name)) counts.searches++;
    else if (WRITE_TOOLS.has(tool.name)) counts.edits++;
    else if (tool.name === "bash" || tool.isShell) counts.commands++;
    else if (DELEGATE_TOOLS.has(tool.name)) counts.delegates++;
  }
  return counts;
}

function dominantPhase(
  tools: ToolItem[],
  hasReasoning: boolean,
  active: boolean,
): ThinkingPhase {
  if (tools.length === 0) {
    return hasReasoning || active ? "planning" : "working";
  }

  const pool = tools.some((t) => t.status === "running")
    ? tools.filter((t) => t.status === "running")
    : tools.slice(-3);

  const score: Record<ThinkingPhase, number> = {
    planning: hasReasoning && active ? 1 : 0,
    exploring: 0,
    reading: 0,
    editing: 0,
    verifying: 0,
    delegating: 0,
    working: 0,
  };

  for (const tool of pool) {
    if (SEARCH_TOOLS.has(tool.name)) score.exploring += 2;
    if (READ_TOOLS.has(tool.name)) score.reading += 2;
    if (WRITE_TOOLS.has(tool.name)) score.editing += 3;
    if (tool.name === "bash" || tool.isShell) {
      const cmd = subjectOf(tool.name, tool.args).toLowerCase();
      if (/test|verify|lint|check|build|npm run|go test|pytest/.test(cmd)) score.verifying += 3;
      else score.working += 1;
    }
    if (DELEGATE_TOOLS.has(tool.name)) score.delegating += 2;
  }

  let best: ThinkingPhase = "working";
  let bestScore = -1;
  for (const [phase, value] of Object.entries(score) as [ThinkingPhase, number][]) {
    if (value > bestScore) {
      best = phase;
      bestScore = value;
    }
  }
  if (bestScore <= 0 && hasReasoning && active) return "planning";
  return best;
}

export function deriveThinkingBlockTitle(block: ThinkingBlock, t: Translator): string {
  const tools = collectToolItems(block.entries);
  const active = thinkingBlockIsActive(block);
  const phase = dominantPhase(tools, block.reasoning.trim().length > 0, active);
  return t(`thinkingBlock.title.${phase}` as DictKey);
}

export function deriveThinkingBlockHint(block: ThinkingBlock, t: Translator): string | null {
  const tools = collectToolItems(block.entries);
  const active = thinkingBlockIsActive(block);
  const hasReasoning = block.reasoning.trim().length > 0;

  if (active && !hasReasoning && tools.length === 0) {
    return t("thinkingBlock.hint.planning");
  }

  const running = tools.filter((tool) => tool.status === "running");
  if (active && running.length > 0) {
    const current = running[running.length - 1]!;
    const subject = subjectOf(current.name, current.args);
    if (current.name === "grep" || current.name === "glob") {
      const target = subject ? basename(subject) || subject : "";
      return target
        ? t("thinkingBlock.hint.searching", { target })
        : t("thinkingBlock.hint.searchingGeneric");
    }
    if (current.name === "read_file" || current.name === "web_fetch") {
      const file = subject ? basename(subject) || subject : t("thinkingBlock.hint.fileGeneric");
      return t("thinkingBlock.hint.reading", { file });
    }
    if (WRITE_TOOLS.has(current.name)) {
      const file = subject ? basename(subject) || subject : t("thinkingBlock.hint.fileGeneric");
      return t("thinkingBlock.hint.editing", { file });
    }
    if (current.name === "bash" || current.isShell) {
      const cmd = subject.trim();
      const short = cmd.length > 48 ? `${cmd.slice(0, 45)}…` : cmd;
      return short ? t("thinkingBlock.hint.command", { command: short }) : t("thinkingBlock.hint.commandGeneric");
    }
    if (DELEGATE_TOOLS.has(current.name)) {
      const label = subject.trim();
      return label
        ? t("thinkingBlock.hint.delegating", { task: label.length > 40 ? `${label.slice(0, 37)}…` : label })
        : t("thinkingBlock.hint.delegatingGeneric");
    }
  }

  if (!active && tools.length >= 2) {
    return summarizeThinkingBlockStats(tools, t);
  }

  return null;
}

export function summarizeThinkingBlockStats(tools: ToolItem[], t: Translator): string | null {
  const counts = countByKind(tools);
  const parts: string[] = [];
  if (counts.reads > 0) parts.push(t("thinkingBlock.stat.reads", { n: counts.reads }));
  if (counts.searches > 0) parts.push(t("thinkingBlock.stat.searches", { n: counts.searches }));
  if (counts.edits > 0) parts.push(t("thinkingBlock.stat.edits", { n: counts.edits }));
  if (counts.commands > 0) parts.push(t("thinkingBlock.stat.commands", { n: counts.commands }));
  if (counts.delegates > 0) parts.push(t("thinkingBlock.stat.delegates", { n: counts.delegates }));
  return parts.length > 0 ? parts.join(" · ") : null;
}

export function verbLabelForTool(name: string, status: ToolStatus, t: Translator): string {
  const running = status === "running";
  const bucket = running ? "thinkingBlock.verb" : "thinkingBlock.verbDone";
  const key = (() => {
    switch (name) {
      case "grep":
        return `${bucket}.grep`;
      case "glob":
        return `${bucket}.glob`;
      case "ls":
        return `${bucket}.ls`;
      case "read_file":
        return `${bucket}.readFile`;
      case "web_fetch":
        return `${bucket}.webFetch`;
      case "edit_file":
      case "write_file":
      case "multi_edit":
        return `${bucket}.edit`;
      case "bash":
        return `${bucket}.bash`;
      case "task":
      case "run_skill":
      case "explore":
      case "research":
      case "security_review":
        return `${bucket}.delegate`;
      case "complete_step":
        return `${bucket}.completeStep`;
      case "todo_write":
        return `${bucket}.todoWrite`;
      default:
        return `${bucket}.generic`;
    }
  })();
  return t(key as DictKey);
}

export function actionContextForTool(item: ToolItem, t: Translator): string | null {
  const args = parseToolArgs(item.args);
  switch (item.name) {
    case "grep": {
      const pattern = toolArgString(args, "pattern");
      const path = toolArgString(args, "path");
      if (pattern && path) return t("thinkingBlock.context.grepIn", { pattern, path });
      if (pattern) return t("thinkingBlock.context.grepPattern", { pattern });
      return null;
    }
    case "glob": {
      const pattern = toolArgString(args, "pattern");
      return pattern ? t("thinkingBlock.context.globPattern", { pattern }) : null;
    }
    case "read_file":
    case "edit_file":
    case "write_file": {
      const path = toolArgString(args, "path") || toolArgString(args, "file_path");
      return path ? t("thinkingBlock.context.path", { path }) : null;
    }
    case "bash": {
      const command = toolArgString(args, "command");
      if (!command) return null;
      const short = command.length > 56 ? `${command.slice(0, 53)}…` : command;
      return t("thinkingBlock.context.command", { command: short });
    }
    case "task":
    case "run_skill": {
      const desc = toolArgString(args, "description") || toolArgString(args, "prompt");
      if (!desc) return null;
      const short = desc.length > 48 ? `${desc.slice(0, 45)}…` : desc;
      return t("thinkingBlock.context.task", { task: short });
    }
    default:
      return null;
  }
}
