import {
  getStoredCodeWorkspaceRoot,
  isProjectLikeCodeWorkspaceRoot,
  sameWorkspaceRoot,
} from "./composerWorkspace";
import { NO_WORKSPACE_VALUE, isNoWriteWorkspace, isUsableWriteWorkspaceRoot } from "./writeWorkspace";
import type { TabMeta } from "./types";

/** Reserved topic id — one writing conversation per workspace, hidden from the code sidebar tree. */
export const WRITE_MODE_TOPIC_ID = "__arcdesk_write__";

export function isWriteModeTopicId(topicId: string | undefined | null): boolean {
  return (topicId ?? "").trim() === WRITE_MODE_TOPIC_ID;
}

export type WriteAgentWorkspaceOptions = {
  /** Persisted code-area workspace (localStorage). */
  codeWorkspaceRoot?: string;
  /** Code tab the user was on before entering write mode. */
  rememberedCodeTab?: TabMeta;
  /** Open project tabs — used as fallback when stored root is missing or too broad. */
  openProjectTabs?: TabMeta[];
};

function pushCandidate(candidates: string[], path?: string) {
  const p = (path ?? "").trim();
  if (!isProjectLikeCodeWorkspaceRoot(p)) return;
  if (candidates.some((c) => sameWorkspaceRoot(c, p))) return;
  candidates.push(p);
}

/** Resolve the code project root Agent tools should use in 写作 mode. */
export function resolveWriteAgentWorkspaceRoot(opts: WriteAgentWorkspaceOptions = {}): string {
  const candidates: string[] = [];
  pushCandidate(candidates, opts.rememberedCodeTab?.workspaceRoot);
  pushCandidate(candidates, opts.codeWorkspaceRoot ?? getStoredCodeWorkspaceRoot());
  for (const tab of opts.openProjectTabs ?? []) {
    if (tab.scope !== "project" || isWriteModeTopicId(tab.topicId)) continue;
    pushCandidate(candidates, tab.workspaceRoot);
  }
  return candidates[0] ?? NO_WORKSPACE_VALUE;
}

/** Agent workspace for 写作 mode: linked code project only, never the document save folder. */
export function writeAgentWorkspaceRoot(
  _writeWorkspaceRoot?: string,
  codeWorkspaceRoot?: string,
  opts: Omit<WriteAgentWorkspaceOptions, "codeWorkspaceRoot"> = {},
): string {
  return resolveWriteAgentWorkspaceRoot({
    ...opts,
    codeWorkspaceRoot: codeWorkspaceRoot ?? getStoredCodeWorkspaceRoot(),
  });
}

export function isWriteTabForWorkspace(
  tab: TabMeta | undefined,
  writeWorkspaceRoot: string,
  codeWorkspaceRoot?: string,
  opts: Omit<WriteAgentWorkspaceOptions, "codeWorkspaceRoot"> = {},
): boolean {
  if (!tab || !isWriteModeTopicId(tab.topicId)) return false;
  const agentRoot = writeAgentWorkspaceRoot(writeWorkspaceRoot, codeWorkspaceRoot, opts);
  if (agentRoot === NO_WORKSPACE_VALUE || !isProjectLikeCodeWorkspaceRoot(agentRoot)) {
    return tab.scope === "global";
  }
  return tab.scope === "project" && sameWorkspaceRoot(tab.workspaceRoot, agentRoot);
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

/** Pick the best code-tab hint for write→code linkage. */
export function pickCodeTabHint(
  tabMetas: TabMeta[],
  rememberedTabId?: string,
  activeTab?: TabMeta,
): TabMeta | undefined {
  if (rememberedTabId) {
    const remembered = tabMetas.find(
      (tab) =>
        tab.id === rememberedTabId &&
        tab.scope === "project" &&
        !isWriteModeTopicId(tab.topicId) &&
        isProjectLikeCodeWorkspaceRoot(tab.workspaceRoot),
    );
    if (remembered) return remembered;
  }
  if (
    activeTab &&
    activeTab.scope === "project" &&
    !isWriteModeTopicId(activeTab.topicId) &&
    isProjectLikeCodeWorkspaceRoot(activeTab.workspaceRoot)
  ) {
    return activeTab;
  }
  return tabMetas.find(
    (tab) =>
      tab.scope === "project" &&
      !isWriteModeTopicId(tab.topicId) &&
      isProjectLikeCodeWorkspaceRoot(tab.workspaceRoot),
  );
}
