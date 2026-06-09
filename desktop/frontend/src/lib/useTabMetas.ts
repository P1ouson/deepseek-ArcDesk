import { useCallback, useEffect, useState } from "react";
import { asArray } from "./array";
import { app, onProjectTreeChanged, onReady } from "./bridge";
import { TAB_METAS_CHANGED_EVENT } from "./events";
import { logBridgeError } from "./logBridgeError";
import type { TabMeta } from "./types";

const FOREGROUND_POLL_MS = 30_000;
const BACKGROUND_POLL_MS = 60_000;

export function useTabMetas() {
  const [tabMetas, setTabMetas] = useState<TabMeta[]>([]);

  const refreshTabMetas = useCallback(async (): Promise<TabMeta[]> => {
    const tabs = asArray(
      await app.ListTabs().catch((err) => {
        logBridgeError("ListTabs", err);
        return [] as TabMeta[];
      }),
    );
    setTabMetas(tabs);
    return tabs;
  }, []);

  useEffect(() => {
    void refreshTabMetas();
    let pollId: ReturnType<typeof setInterval> | undefined;
    const schedulePoll = () => {
      if (pollId !== undefined) window.clearInterval(pollId);
      const intervalMs = typeof document !== "undefined" && document.hidden ? BACKGROUND_POLL_MS : FOREGROUND_POLL_MS;
      pollId = window.setInterval(() => void refreshTabMetas(), intervalMs);
    };
    schedulePoll();
    const onVisibilityChange = () => schedulePoll();
    const onTabMetasChanged = () => void refreshTabMetas();
    document.addEventListener("visibilitychange", onVisibilityChange);
    window.addEventListener(TAB_METAS_CHANGED_EVENT, onTabMetasChanged);
    return () => {
      document.removeEventListener("visibilitychange", onVisibilityChange);
      window.removeEventListener(TAB_METAS_CHANGED_EVENT, onTabMetasChanged);
      if (pollId !== undefined) window.clearInterval(pollId);
    };
  }, [refreshTabMetas]);

  useEffect(() => onReady(() => void refreshTabMetas()), [refreshTabMetas]);

  useEffect(
    () =>
      onProjectTreeChanged(() => {
        void refreshTabMetas();
      }),
    [refreshTabMetas],
  );

  return { tabMetas, refreshTabMetas };
}
