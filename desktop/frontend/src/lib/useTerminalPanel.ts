import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { clampTerminalPanelHeight, TERMINAL_PANEL_DEFAULT_HEIGHT, type TerminalTab } from "../components/TerminalPanel";
import { closeAllTerminals, closeTerminal, startTerminal } from "./terminalBridge";
import { loadLayoutSize, saveLayoutSize } from "./layoutPreferences";
import { usePanelSlide, usePanelSlideMeasure } from "./usePanelSlide";
import { useTabCollection } from "./useTabCollection";

export type TerminalPanelDeps = {
  notice: (text: string, level?: "info" | "warn") => void;
};

export function useTerminalPanel(deps: TerminalPanelDeps) {
  const { notice } = deps;

  const [terminalOpen, setTerminalOpen] = useState(false);
  const { tabs: terminalTabs, activeId, setActiveId, openTab, replaceTabs } = useTabCollection<TerminalTab>();
  const [terminalHeight, setTerminalHeight] = useState(() =>
    loadLayoutSize("terminalPanelHeight", TERMINAL_PANEL_DEFAULT_HEIGHT, clampTerminalPanelHeight),
  );
  const [terminalAnimHeight, setTerminalAnimHeight] = useState(0);
  const [terminalMotionKey, setTerminalMotionKey] = useState(0);
  const terminalTabKeyRef = useRef(0);
  const terminalHeightRef = useRef(terminalHeight);
  const slideWasOpenRef = useRef(false);

  terminalHeightRef.current = terminalHeight;

  const terminalHasSessions = terminalTabs.length > 0;
  const terminalPanelVisible = terminalOpen;
  const slideOpen = terminalOpen && terminalTabs.length > 0;
  const { shown: terminalPanelShown } = usePanelSlide(slideOpen);
  const measureSlide = usePanelSlideMeasure(
    useCallback(() => setTerminalAnimHeight(terminalHeightRef.current), []),
  );

  useEffect(() => {
    if (!slideOpen) {
      slideWasOpenRef.current = false;
      if (!terminalPanelShown) return;
      setTerminalAnimHeight(0);
      return;
    }
    const opening = !slideWasOpenRef.current;
    slideWasOpenRef.current = true;
    if (opening) {
      setTerminalMotionKey((key) => key + 1);
      setTerminalAnimHeight(0);
      return measureSlide();
    }
    setTerminalAnimHeight(terminalHeightRef.current);
  }, [slideOpen, terminalPanelShown, measureSlide]);

  useEffect(() => {
    if (!slideOpen || !terminalPanelShown) return;
    setTerminalAnimHeight(terminalHeight);
  }, [terminalHeight, slideOpen, terminalPanelShown]);

  const closeTerminalPanel = useCallback(() => {
    closeAllTerminals();
    replaceTabs([], null);
    setTerminalOpen(false);
  }, [replaceTabs]);

  const minimizeTerminalPanel = useCallback(() => {
    setTerminalOpen(false);
  }, []);

  const restoreTerminalPanel = useCallback(() => {
    if (terminalTabs.length === 0) return;
    setTerminalOpen(true);
  }, [terminalTabs.length]);

  const openNewTerminal = useCallback(async () => {
    const result = await startTerminal();
    if (result.err) {
      notice(result.err);
      return;
    }
    const shellName = result.shell.split(/[/\\]/).pop() || result.shell;
    terminalTabKeyRef.current += 1;
    const clientKey = `terminal-tab-${terminalTabKeyRef.current}`;
    const title =
      terminalTabs.some((tab) => tab.title === shellName) ? `${shellName} ${terminalTabs.length + 1}` : shellName;
    openTab({ id: result.id, clientKey, title, shell: result.shell });
    setTerminalOpen(true);
  }, [notice, openTab, terminalTabs]);

  const resolvedActiveTerminalId = useMemo(() => {
    if (terminalTabs.length === 0) return null;
    if (activeId && terminalTabs.some((tab) => tab.id === activeId)) {
      return activeId;
    }
    return terminalTabs[terminalTabs.length - 1]?.id ?? null;
  }, [activeId, terminalTabs]);

  useEffect(() => {
    if (!resolvedActiveTerminalId || resolvedActiveTerminalId === activeId) return;
    setActiveId(resolvedActiveTerminalId);
  }, [activeId, resolvedActiveTerminalId, setActiveId]);

  const closeTerminalTab = useCallback(
    (id: string, index?: number) => {
      const removeAt =
        typeof index === "number" && index >= 0 && index < terminalTabs.length && terminalTabs[index]?.id === id
          ? index
          : terminalTabs.findIndex((tab) => tab.id === id);
      if (removeAt === -1) return;

      const next = [...terminalTabs.slice(0, removeAt), ...terminalTabs.slice(removeAt + 1)];
      if (!next.some((tab) => tab.id === id)) {
        closeTerminal(id);
      }

      let nextActiveId = activeId;
      if (next.length === 0) {
        setTerminalOpen(false);
        nextActiveId = null;
      } else if (activeId === id) {
        const fallbackIndex = Math.min(removeAt, next.length - 1);
        nextActiveId = next[fallbackIndex]!.id;
      }
      replaceTabs(next, nextActiveId);
    },
    [activeId, replaceTabs, terminalTabs],
  );

  const setSavedTerminalHeight = useCallback((height: number) => {
    const next = clampTerminalPanelHeight(height);
    setTerminalHeight(next);
    saveLayoutSize("terminalPanelHeight", next, clampTerminalPanelHeight);
  }, []);

  const activeTerminalTab = useMemo(() => {
    if (!resolvedActiveTerminalId) return null;
    return terminalTabs.find((tab) => tab.id === resolvedActiveTerminalId) ?? null;
  }, [resolvedActiveTerminalId, terminalTabs]);

  return {
    terminalOpen,
    terminalHasSessions,
    terminalPanelVisible,
    terminalTabs,
    terminalPanelShown,
    terminalAnimHeight,
    terminalMotionKey,
    activeTerminalId: activeId,
    setActiveTerminalId: setActiveId,
    resolvedActiveTerminalId,
    activeTerminalTab,
    openNewTerminal,
    closeTerminalPanel,
    minimizeTerminalPanel,
    restoreTerminalPanel,
    closeTerminalTab,
    setSavedTerminalHeight,
  };
}
