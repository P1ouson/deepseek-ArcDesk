import { useCallback, useRef } from "react";
import type { AppMode } from "./appMode";
import type { TabMeta } from "./types";
import {
  WRITE_MODE_TOPIC_ID,
  isWriteModeTopicId,
  isWriteTabForWorkspace,
  normalizedWriteWorkspaceRoot,
  writeTabScope,
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

  const rememberCodeTab = useCallback(() => {
    if (!activeTabId || !activeTab) return;
    if (isWriteModeTopicId(activeTab.topicId)) return;
    codeTabIdBeforeWriteRef.current = activeTabId;
  }, [activeTab, activeTabId]);

  const activateWriteTab = useCallback(
    async (writeRoot: string) => {
      if (activatingRef.current) return;
      activatingRef.current = true;
      try {
        rememberCodeTab();
        const normalized = normalizedWriteWorkspaceRoot(writeRoot);
        if (writeTabScope(normalized) === "project") {
          await openProjectTab(normalized, WRITE_MODE_TOPIC_ID);
        } else {
          await openGlobalTab(WRITE_MODE_TOPIC_ID);
        }
      } finally {
        activatingRef.current = false;
      }
    },
    [openGlobalTab, openProjectTab, rememberCodeTab],
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
      if (isWriteTabForWorkspace(activeTab, writeRoot)) return;
      await activateWriteTab(writeRoot);
    },
    [activateWriteTab, activeTab],
  );

  return {
    activateWriteTab,
    restoreCodeTab,
    ensureWriteTabMatchesWorkspace,
  };
}
