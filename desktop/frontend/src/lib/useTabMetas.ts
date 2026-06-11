import { useCallback, useEffect, useState } from "react";
import { asArray } from "./array";
import { app, onProjectTreeChanged, onReady } from "./bridge";
import { TAB_METAS_CHANGED_EVENT } from "./events";
import { logBridgeError } from "./logBridgeError";
import type { TabMeta } from "./types";

const FOREGROUND_POLL_MS = 30_000;
const BACKGROUND_POLL_MS = 60_000;
const BOOT_POLL_MS = 150;

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
    let bootPollId: ReturnType<typeof setInterval> | undefined;
    const schedulePoll = () => {
      if (pollId !== undefined) window.clearInterval(pollId);
      const intervalMs = typeof document !== "undefined" && document.hidden ? BACKGROUND_POLL_MS : FOREGROUND_POLL_MS;
      pollId = window.setInterval(() => void refreshTabMetas(), intervalMs);
    };
    schedulePoll();
    bootPollId = window.setInterval(() => {
      void refreshTabMetas().then((tabs) => {
        if (tabs.some((tab) => tab.ready || tab.startupErr)) {
          if (bootPollId !== undefined) window.clearInterval(bootPollId);
          bootPollId = undefined;
        }
      });
    }, BOOT_POLL_MS);
    const onVisibilityChange = () => schedulePoll();
    const onTabMetasChanged = () => void refreshTabMetas();
    document.addEventListener("visibilitychange", onVisibilityChange);
    window.addEventListener(TAB_METAS_CHANGED_EVENT, onTabMetasChanged);
    return () => {
      document.removeEventListener("visibilitychange", onVisibilityChange);
      window.removeEventListener(TAB_METAS_CHANGED_EVENT, onTabMetasChanged);
      if (pollId !== undefined) window.clearInterval(pollId);
      if (bootPollId !== undefined) window.clearInterval(bootPollId);
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
