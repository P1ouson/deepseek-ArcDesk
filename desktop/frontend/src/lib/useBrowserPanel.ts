import { useCallback, useMemo, useRef } from "react";
import { browserTabTitle } from "./browserTabTitle";
import { useTabCollection } from "./useTabCollection";
import { defaultPreviewUrl } from "./webPreviewUrl";

export interface BrowserTab {
  id: string;
  clientKey: string;
  url: string;
  title: string;
}

export function useBrowserPanel() {
  const { tabs: browserTabs, activeId, setActiveId, openTab, closeTab, updateTab, replaceTabs } =
    useTabCollection<BrowserTab>();
  const tabKeyRef = useRef(0);

  const browserActive = browserTabs.length > 0;

  const openBrowserTab = useCallback(
    (url?: string) => {
      const href = url?.trim() || defaultPreviewUrl();
      tabKeyRef.current += 1;
      const clientKey = `browser-${tabKeyRef.current}`;
      const id = `browser-${Date.now()}-${tabKeyRef.current}`;
      const title = browserTabTitle(href, browserTabs.length);
      openTab({ id, clientKey, url: href, title });
      return id;
    },
    [browserTabs.length, openTab],
  );

  const updateBrowserTab = useCallback(
    (id: string, patch: Partial<Pick<BrowserTab, "url" | "title">>) => {
      updateTab(id, patch);
    },
    [updateTab],
  );

  const closeBrowserTab = useCallback(
    (id: string) => {
      closeTab(id);
    },
    [closeTab],
  );

  const closeAllBrowserTabs = useCallback(() => {
    replaceTabs([], null);
  }, [replaceTabs]);

  const resolvedActiveBrowserTabId = useMemo(() => {
    if (browserTabs.length === 0) return null;
    if (activeId && browserTabs.some((tab) => tab.id === activeId)) {
      return activeId;
    }
    return browserTabs[browserTabs.length - 1]?.id ?? null;
  }, [activeId, browserTabs]);

  const activeBrowserTab = useMemo(() => {
    if (!resolvedActiveBrowserTabId) return null;
    return browserTabs.find((tab) => tab.id === resolvedActiveBrowserTabId) ?? null;
  }, [browserTabs, resolvedActiveBrowserTabId]);

  return {
    browserTabs,
    browserActive,
    activeBrowserTab,
    activeBrowserTabId: resolvedActiveBrowserTabId,
    setActiveBrowserTabId: setActiveId,
    openBrowserTab,
    updateBrowserTab,
    closeBrowserTab,
    closeAllBrowserTabs,
  };
}
