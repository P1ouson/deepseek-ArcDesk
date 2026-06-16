import { useCallback, useRef } from "react";
import type { AppMode } from "./appMode";
import { getStoredCodeWorkspaceRoot, isProjectLikeCodeWorkspaceRoot } from "./composerWorkspace";
import type { TabMeta } from "./types";
import type { WriteAgentWorkspaceOptions } from "./writeTab";
import {
  WRITE_MODE_TOPIC_ID,
  isWriteModeTopicId,
  isWriteTabForWorkspace,
  pickCodeTabHint,
  resolveWriteAgentWorkspaceRoot,
} from "./writeTab";

type Params = {
  appMode: AppMode;
  writeWorkspaceRoot: string;
  activeTabId?: string;
  activeTab?: TabMeta;
  tabMetas: TabMeta[];
  openProjectTab: (workspaceRoot: string, topicId: string) => Promise<TabMeta | undefined>;
  openGlobalTab: (topicId: string) => Promise<TabMeta | undefined>;
  switchTab: (tabId: string) => Promise<void>;
  syncActiveTab: (reset?: boolean) => Promise<string | undefined>;
  refreshTabMetas: () => Promise<TabMeta[]>;
};

export function useWriteModeTab({
  activeTabId,
  activeTab,
  tabMetas,
  openProjectTab,
  openGlobalTab,
  switchTab,
  syncActiveTab,
}: Params) {
  const codeTabIdBeforeWriteRef = useRef<string | undefined>();
  const activatingRef = useRef(false);

  const getWriteAgentWorkspaceOpts = useCallback((): Omit<WriteAgentWorkspaceOptions, "codeWorkspaceRoot"> => {
    return {
      rememberedCodeTab: pickCodeTabHint(tabMetas, codeTabIdBeforeWriteRef.current, activeTab),
      openProjectTabs: tabMetas,
    };
  }, [activeTab, tabMetas]);

  const rememberCodeTab = useCallback(() => {
    if (!activeTabId || !activeTab) return;
    if (isWriteModeTopicId(activeTab.topicId)) return;
    codeTabIdBeforeWriteRef.current = activeTabId;
  }, [activeTab, activeTabId]);

  const activateWriteTab = useCallback(
    async (_writeRoot: string) => {
      if (activatingRef.current) return;
      activatingRef.current = true;
      try {
        rememberCodeTab();
        const agentRoot = resolveWriteAgentWorkspaceRoot({
          ...getWriteAgentWorkspaceOpts(),
          codeWorkspaceRoot: getStoredCodeWorkspaceRoot(),
        });
        if (isProjectLikeCodeWorkspaceRoot(agentRoot)) {
          await openProjectTab(agentRoot, WRITE_MODE_TOPIC_ID);
          return;
        }
        await openGlobalTab(WRITE_MODE_TOPIC_ID);
      } finally {
        activatingRef.current = false;
      }
    },
    [getWriteAgentWorkspaceOpts, openGlobalTab, openProjectTab, rememberCodeTab],
  );

  const restoreCodeTab = useCallback(async () => {
    const saved = codeTabIdBeforeWriteRef.current;
    codeTabIdBeforeWriteRef.current = undefined;
    if (saved && tabMetas.some((tab) => tab.id === saved && !isWriteModeTopicId(tab.topicId))) {
      await switchTab(saved);
      return;
    }
    await syncActiveTab(false);
  }, [switchTab, syncActiveTab, tabMetas]);

  const ensureWriteTabMatchesWorkspace = useCallback(
    async (writeRoot: string) => {
      const opts = getWriteAgentWorkspaceOpts();
      const codeRoot = getStoredCodeWorkspaceRoot();
      if (isWriteTabForWorkspace(activeTab, writeRoot, codeRoot, opts)) return;
      await activateWriteTab(writeRoot);
    },
    [activateWriteTab, activeTab, getWriteAgentWorkspaceOpts],
  );

  return {
    activateWriteTab,
    restoreCodeTab,
    ensureWriteTabMatchesWorkspace,
    getWriteAgentWorkspaceOpts,
  };
}
