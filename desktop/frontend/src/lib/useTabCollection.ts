import { useCallback, useMemo, useState } from "react";

export type TabCollection<T extends { id: string }> = {
  tabs: T[];
  activeId: string | null;
  setActiveId: (id: string | null) => void;
  openTab: (tab: T) => void;
  closeTab: (id: string) => void;
  updateTab: (id: string, patch: Partial<T>) => void;
  replaceTabs: (next: T[], activeId?: string | null) => void;
};

/** Shared tab strip state for browser/terminal panels. */
export function useTabCollection<T extends { id: string }>(
  initialTabs: T[] = [],
  initialActiveId: string | null = null,
): TabCollection<T> {
  const [tabs, setTabs] = useState(initialTabs);
  const [activeId, setActiveId] = useState<string | null>(initialActiveId);

  const openTab = useCallback((tab: T) => {
    setTabs((current) => {
      const idx = current.findIndex((entry) => entry.id === tab.id);
      if (idx >= 0) {
        const next = [...current];
        next[idx] = tab;
        return next;
      }
      return [...current, tab];
    });
    setActiveId(tab.id);
  }, []);

  const closeTab = useCallback((id: string) => {
    setTabs((current) => {
      const removeAt = current.findIndex((tab) => tab.id === id);
      if (removeAt < 0) return current;
      const next = current.filter((tab) => tab.id !== id);
      setActiveId((active) => {
        if (active !== id) return active;
        const fallbackIndex = Math.min(removeAt, Math.max(0, next.length - 1));
        return next[fallbackIndex]?.id ?? null;
      });
      return next;
    });
  }, []);

  const updateTab = useCallback((id: string, patch: Partial<T>) => {
    setTabs((current) => current.map((tab) => (tab.id === id ? { ...tab, ...patch } : tab)));
  }, []);

  const replaceTabs = useCallback((next: T[], nextActiveId?: string | null) => {
    setTabs(next);
    setActiveId(nextActiveId ?? next[0]?.id ?? null);
  }, []);

  return useMemo(
    () => ({ tabs, activeId, setActiveId, openTab, closeTab, updateTab, replaceTabs }),
    [tabs, activeId, openTab, closeTab, updateTab, replaceTabs],
  );
}
