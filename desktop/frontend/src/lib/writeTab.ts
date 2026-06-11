import { sameWorkspaceRoot } from "./composerWorkspace";
import { NO_WORKSPACE_VALUE, isNoWriteWorkspace, isUsableWriteWorkspaceRoot } from "./writeWorkspace";
import type { TabMeta } from "./types";

/** Reserved topic id — one writing conversation per workspace, hidden from the code sidebar tree. */
export const WRITE_MODE_TOPIC_ID = "__arcdesk_write__";

export function isWriteModeTopicId(topicId: string | undefined | null): boolean {
  return (topicId ?? "").trim() === WRITE_MODE_TOPIC_ID;
}

export function isWriteTabForWorkspace(tab: TabMeta | undefined, writeWorkspaceRoot: string): boolean {
  if (!tab || !isWriteModeTopicId(tab.topicId)) return false;
  if (isNoWriteWorkspace(writeWorkspaceRoot) || !isUsableWriteWorkspaceRoot(writeWorkspaceRoot)) {
    return tab.scope === "global";
  }
  return tab.scope === "project" && sameWorkspaceRoot(tab.workspaceRoot, writeWorkspaceRoot);
}

export function writeTabScope(writeWorkspaceRoot: string): "global" | "project" {
  if (isNoWriteWorkspace(writeWorkspaceRoot) || !isUsableWriteWorkspaceRoot(writeWorkspaceRoot)) {
    return "global";
  }
  return "project";
}

export function normalizedWriteWorkspaceRoot(writeWorkspaceRoot: string): string {
  if (isNoWriteWorkspace(writeWorkspaceRoot)) return NO_WORKSPACE_VALUE;
  return writeWorkspaceRoot.trim();
}
