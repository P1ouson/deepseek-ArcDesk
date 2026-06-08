import { getCommitInstructions, getPRInstructions, getPRMergeMethod, ghPRMergeFlag } from "./desktopGitPrefs";
import type { useT } from "./i18n";

function mergeMethodLabel(method: ReturnType<typeof getPRMergeMethod>, t: ReturnType<typeof useT>): string {
  switch (method) {
    case "squash":
      return t("settings.git.mergeMethod.squash");
    case "rebase":
      return t("settings.git.mergeMethod.rebase");
    default:
      return t("settings.git.mergeMethod.merge");
  }
}

export function buildCommitMessagePrompt(
  files: string,
  t: ReturnType<typeof useT>,
): string {
  const base = t("git.suggestCommitPrompt", { files });
  const instructions = getCommitInstructions();
  if (!instructions) return base;
  return `${base}\n\n${instructions}`;
}

export function buildPRPrompt(context: string, t: ReturnType<typeof useT>): string {
  const method = getPRMergeMethod();
  const mergeHint = t("git.prMergePreference", { method: mergeMethodLabel(method, t) });
  const base = t("git.suggestPRPrompt", { context: `${context}\n\n${mergeHint}` });
  const instructions = getPRInstructions();
  if (!instructions) return base;
  return `${base}\n\n${instructions}`;
}

export function ghPRMergeCommand(): string {
  return `gh pr merge ${ghPRMergeFlag()} --yes`;
}
