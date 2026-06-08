import type { ComposerWriteContext } from "./types";

const PROMPT_SUFFIX: Record<string, { selection: string; document: string }> = {
  "write.action.summarize": {
    selection: "write.action.summarizePromptSelection",
    document: "write.action.summarizePromptDocument",
  },
  "write.action.outline": {
    selection: "write.action.outlinePromptSelection",
    document: "write.action.outlinePromptDocument",
  },
  "write.action.polish": {
    selection: "write.action.polishPromptSelection",
    document: "write.action.polishPromptDocument",
  },
  "write.action.expand": {
    selection: "write.action.expandPromptSelection",
    document: "write.action.expandPromptDocument",
  },
  "write.action.shorten": {
    selection: "write.action.shortenPromptSelection",
    document: "write.action.shortenPromptDocument",
  },
  "write.action.proofread": {
    selection: "write.action.proofreadPromptSelection",
    document: "write.action.proofreadPromptDocument",
  },
};

export function writeActionPromptKey(actionKey: string, scope: ComposerWriteContext["scope"]): string {
  const entry = PROMPT_SUFFIX[actionKey];
  if (!entry) return `${actionKey}Prompt`;
  return scope === "selection" ? entry.selection : entry.document;
}
