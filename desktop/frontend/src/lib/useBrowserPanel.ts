import { useCallback, useMemo, useRef, useState } from "react";
import { browserTabTitle } from "./browserTabTitle";
import { defaultPreviewUrl } from "./webPreviewUrl";

export interface BrowserTab {
  id: string;
  clientKey: string;
  url: string;
  title: string;
}

export function useBrowserPanel() {
  const [browserTabs, setBrowserTabs] = useState<BrowserTab[]>([]);
  const [activeBrowserTabId, setActiveBrowserTabId] = useState<string | null>(null);
  const tabKeyRef = useRef(0);

  const browserActive = browserTabs.length > 0;

  const openBrowserTab = useCallback((url?: string) => {
    const href = url?.trim() || defaultPreviewUrl();
    tabKeyRef.current += 1;
    const clientKey = `browser-${tabKeyRef.current}`;
    const id = `browser-${Date.now()}-${tabKeyRef.current}`;
    setBrowserTabs((current) => {
      const title = browserTabTitle(href, current.length);
      return [...current, { id, clientKey, url: href, title }];
    });
    setActiveBrowserTabId(id);
    return id;
  }, []);

  const updateBrowserTab = useCallback((id: string, patch: Partial<Pick<BrowserTab, "url" | "title">>) => {
    setBrowserTabs((current) => current.map((tab) => (tab.id === id ? { ...tab, ...patch } : tab)));
  }, []);

  const closeBrowserTab = useCallback((id: string) => {
    setBrowserTabs((current) => {
      const removeAt = current.findIndex((tab) => tab.id === id);
      if (removeAt === -1) return current;
      const next = current.filter((tab) => tab.id !== id);
      setActiveBrowserTabId((active) => {
        if (active !== id) return active;
        const fallbackIndex = Math.min(removeAt, Math.max(0, next.length - 1));
        return next[fallbackIndex]?.id ?? null;
      });
      return next;
    });
  }, []);

  const closeAllBrowserTabs = useCallback(() => {
    setBrowserTabs([]);
    setActiveBrowserTabId(null);
  }, []);

  const resolvedActiveBrowserTabId = useMemo(() => {
    if (browserTabs.length === 0) return null;
    if (activeBrowserTabId && browserTabs.some((tab) => tab.id === activeBrowserTabId)) {
      return activeBrowserTabId;
    }
    return browserTabs[browserTabs.length - 1]?.id ?? null;
  }, [activeBrowserTabId, browserTabs]);

  const activeBrowserTab = useMemo(() => {
    if (!resolvedActiveBrowserTabId) return null;
    return browserTabs.find((tab) => tab.id === resolvedActiveBrowserTabId) ?? null;
  }, [browserTabs, resolvedActiveBrowserTabId]);

  return {
    browserTabs,
    browserActive,
    activeBrowserTab,
    activeBrowserTabId: resolvedActiveBrowserTabId,
    setActiveBrowserTabId,
    openBrowserTab,
    updateBrowserTab,
    closeBrowserTab,
    closeAllBrowserTabs,
  };
}
