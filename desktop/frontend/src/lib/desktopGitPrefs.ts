import type { DesktopGitView } from "./types";

const DEFAULT_GIT: DesktopGitView = {
  prMergeMethod: "merge",
  checkGitHubCli: false,
  syncRepoMergeToGitHub: false,
  commitInstructions: "",
  prInstructions: "",
};

let current: DesktopGitView = { ...DEFAULT_GIT };

export function normalizeDesktopGit(git: Partial<DesktopGitView> | undefined | null): DesktopGitView {
  const method = git?.prMergeMethod;
  return {
    prMergeMethod: method === "squash" || method === "rebase" ? method : "merge",
    checkGitHubCli: git?.checkGitHubCli === true,
    syncRepoMergeToGitHub: git?.syncRepoMergeToGitHub === true,
    commitInstructions: git?.commitInstructions ?? "",
    prInstructions: git?.prInstructions ?? "",
  };
}

export function syncDesktopGitSettings(git: Partial<DesktopGitView> | undefined | null): DesktopGitView {
  current = normalizeDesktopGit(git);
  if (typeof document === "undefined") return current;
  const root = document.documentElement;
  root.setAttribute("data-git-pr-merge-method", current.prMergeMethod);
  root.setAttribute("data-git-check-github-cli", current.checkGitHubCli ? "1" : "0");
  root.setAttribute("data-git-sync-repo-merge", current.syncRepoMergeToGitHub ? "1" : "0");
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("ARCDESK:desktop-git-settings"));
  }
  return current;
}

export function getDesktopGitSettings(): DesktopGitView {
  return current;
}

export function isGitHubCliCheckEnabled(): boolean {
  return current.checkGitHubCli;
}

export function isSyncRepoMergeToGitHubEnabled(): boolean {
  return current.syncRepoMergeToGitHub;
}

export function getCommitInstructions(): string {
  return current.commitInstructions.trim();
}

export function getPRInstructions(): string {
  return current.prInstructions.trim();
}

export function getPRMergeMethod(): DesktopGitView["prMergeMethod"] {
  return current.prMergeMethod;
}

/** gh CLI flag for `gh pr merge` matching the configured merge method. */
export function ghPRMergeFlag(method: DesktopGitView["prMergeMethod"] = getPRMergeMethod()): string {
  switch (method) {
    case "squash":
      return "--squash";
    case "rebase":
      return "--rebase";
    default:
      return "--merge";
  }
}
