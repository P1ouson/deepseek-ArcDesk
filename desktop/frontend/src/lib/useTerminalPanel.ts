import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { clampTerminalPanelHeight, TERMINAL_PANEL_DEFAULT_HEIGHT, type TerminalTab } from "../components/TerminalPanel";
import { closeAllTerminals, closeTerminal, startTerminal } from "./terminalBridge";
import { loadLayoutSize, saveLayoutSize } from "./layoutPreferences";

/** Panel slide duration — keep in sync with --duration-normal in design-system.css */
const MOTION_PANEL_MS = 220;

export type TerminalPanelDeps = {
  notice: (text: string, level?: "info" | "warn") => void;
};

export function useTerminalPanel(deps: TerminalPanelDeps) {
  const { notice } = deps;

  const [terminalOpen, setTerminalOpen] = useState(false);
  const [terminalTabs, setTerminalTabs] = useState<TerminalTab[]>([]);
  const [activeTerminalId, setActiveTerminalId] = useState<string | null>(null);
  const [terminalHeight, setTerminalHeight] = useState(() =>
    loadLayoutSize("terminalPanelHeight", TERMINAL_PANEL_DEFAULT_HEIGHT, clampTerminalPanelHeight),
  );
  const [terminalPanelShown, setTerminalPanelShown] = useState(false);
  const [terminalAnimHeight, setTerminalAnimHeight] = useState(0);
  const [terminalMotionKey, setTerminalMotionKey] = useState(0);
  const terminalCloseTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const terminalTabKeyRef = useRef(0);

  useEffect(() => {
    if (!terminalOpen) {
      if (!terminalPanelShown) return;
      setTerminalAnimHeight(0);
      if (terminalCloseTimerRef.current) clearTimeout(terminalCloseTimerRef.current);
      terminalCloseTimerRef.current = setTimeout(() => {
        setTerminalPanelShown(false);
        terminalCloseTimerRef.current = null;
      }, MOTION_PANEL_MS);
      return () => {
        if (terminalCloseTimerRef.current) {
          clearTimeout(terminalCloseTimerRef.current);
          terminalCloseTimerRef.current = null;
        }
      };
    }
    if (terminalTabs.length === 0) return;
    if (terminalCloseTimerRef.current) {
      clearTimeout(terminalCloseTimerRef.current);
      terminalCloseTimerRef.current = null;
    }
    setTerminalMotionKey((key) => key + 1);
    setTerminalPanelShown(true);
    setTerminalAnimHeight(0);
    const id = window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        setTerminalAnimHeight(terminalHeight);
      });
    });
    return () => window.cancelAnimationFrame(id);
  }, [terminalOpen, terminalTabs.length]);

  useEffect(() => {
    if (!terminalOpen || !terminalPanelShown || terminalAnimHeight <= 0) return;
    setTerminalAnimHeight(terminalHeight);
  }, [terminalHeight, terminalOpen, terminalPanelShown, terminalAnimHeight]);

  const closeTerminalPanel = useCallback(() => {
    closeAllTerminals();
    setTerminalTabs([]);
    setActiveTerminalId(null);
    setTerminalOpen(false);
  }, []);

  const openNewTerminal = useCallback(async () => {
    const result = await startTerminal();
    if (result.err) {
      notice(result.err);
      return;
    }
    const shellName = result.shell.split(/[/\\]/).pop() || result.shell;
    terminalTabKeyRef.current += 1;
    const clientKey = `terminal-tab-${terminalTabKeyRef.current}`;
    setTerminalTabs((current) => {
      const title =
        current.some((tab) => tab.title === shellName) ? `${shellName} ${current.length + 1}` : shellName;
      return [...current, { id: result.id, clientKey, title, shell: result.shell }];
    });
    setActiveTerminalId(result.id);
    setTerminalOpen(true);
  }, [notice]);

  const resolvedActiveTerminalId = useMemo(() => {
    if (terminalTabs.length === 0) return null;
    if (activeTerminalId && terminalTabs.some((tab) => tab.id === activeTerminalId)) {
      return activeTerminalId;
    }
    return terminalTabs[terminalTabs.length - 1]?.id ?? null;
  }, [activeTerminalId, terminalTabs]);

  useEffect(() => {
    if (!resolvedActiveTerminalId || resolvedActiveTerminalId === activeTerminalId) return;
    setActiveTerminalId(resolvedActiveTerminalId);
  }, [activeTerminalId, resolvedActiveTerminalId]);

  const closeTerminalTab = useCallback((id: string, index?: number) => {
    setTerminalTabs((current) => {
      const removeAt =
        typeof index === "number" && index >= 0 && index < current.length && current[index]?.id === id
          ? index
          : current.findIndex((tab) => tab.id === id);
      if (removeAt === -1) return current;
      const next = [...current.slice(0, removeAt), ...current.slice(removeAt + 1)];
      if (!next.some((tab) => tab.id === id)) {
        closeTerminal(id);
      }
      if (next.length === 0) {
        setTerminalOpen(false);
        setActiveTerminalId(null);
      } else {
        setActiveTerminalId((active) => {
          if (active !== id) return active;
          const fallbackIndex = Math.min(removeAt, next.length - 1);
          return next[fallbackIndex]!.id;
        });
      }
      return next;
    });
  }, []);

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
    terminalTabs,
    terminalPanelShown,
    terminalAnimHeight,
    terminalMotionKey,
    activeTerminalId,
    setActiveTerminalId,
    resolvedActiveTerminalId,
    activeTerminalTab,
    openNewTerminal,
    closeTerminalPanel,
    closeTerminalTab,
    setSavedTerminalHeight,
  };
}
