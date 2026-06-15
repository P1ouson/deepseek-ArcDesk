import type { Translator } from "./i18n";

/** Human-readable tooltip for `git status --porcelain` two-letter codes. */
export function gitStatusTooltip(status: string, t: Translator): string {
  const code = status.trim().toUpperCase();
  if (code === "??") return t("git.statusUntracked");
  if (code.includes("D")) return t("git.statusDeleted");
  if (code.includes("A") && !code.includes("M")) return t("git.statusAdded");
  if (code.includes("R")) return t("git.statusRenamed");
  if (code.includes("M")) return t("git.statusModified");
  return t("git.statusOther", { code: status.trim() });
}
