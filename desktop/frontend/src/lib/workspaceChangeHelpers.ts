import type { WorkspaceChangeView } from "./types";

export function hasGitChange(row: WorkspaceChangeView): boolean {
  return row.sources.includes("git");
}

export function isDeletedGitChange(row: WorkspaceChangeView): boolean {
  return !!row.gitStatus && row.gitStatus.includes("D");
}
