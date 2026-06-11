import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type Dispatch,
  type KeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type SetStateAction,
} from "react";
import type { RightDockTab } from "../components/Topbar";
import type { ActionFileOpenRequest } from "../components/ActionStream";
import type { AppMode } from "./appMode";
import {
  dockHubForTab,
  loadHubLastTab,
  loadPreviewPanelState,
  resolveHubTab,
  saveDockTabSelection,
  savePreviewPanelState,
  loadPreviewHubTab,
  type DockHub,
  type PreviewMode,
} from "./dockHubs";
import { loadLayoutSize, loadOptionalLayoutSize, saveLayoutSize } from "./layoutPreferences";
import type { ToolFileDiff } from "./tools";
import type { ComposerInsertRequest } from "./types";
import { isPreviewablePagePath } from "./previewPage";

const STUDIO_RAIL_WIDTH = 76;
const STUDIO_DRAWER_WIDTH = 280;
const CHAT_MIN_WIDTH = 760;
/** Panel slide duration — keep in sync with --duration-normal in design-system.css */
const MOTION_PANEL_MS = 220;

/** Minimum chat width reserved when sizing the right dock — lower than CHAT_MIN_WIDTH so the dock can open with the project drawer visible. */
const DOCK_CHAT_MIN_WIDTH = 420;
const WORKSPACE_RESIZER_WIDTH = 8;
const RIGHT_DOCK_DEFAULT_WIDTH = 380;
const RIGHT_DOCK_DEFAULT_RATIO = 0.28;
const PREVIEW_DOCK_DEFAULT_RATIO = 0.36;
const PREVIEW_DOCK_MIN_WIDTH = 420;
const PREVIEW_DOCK_EXPANDED_RATIO = 0.5;
const RIGHT_DOCK_MIN_WIDTH = 280;
const RIGHT_DOCK_MAX_WIDTH = 720;
const RIGHT_DOCK_MIN_RENDER_WIDTH = 200;
const FILE_PREVIEW_MIN_WIDTH = RIGHT_DOCK_MIN_WIDTH;
const FILE_PREVIEW_MAX_WIDTH = RIGHT_DOCK_MAX_WIDTH;
const FILE_PREVIEW_DEFAULT_WIDTH = RIGHT_DOCK_DEFAULT_WIDTH;

function clampRightDockWidth(width: number): number {
  return Math.min(RIGHT_DOCK_MAX_WIDTH, Math.max(RIGHT_DOCK_MIN_WIDTH, Math.round(width)));
}

function clampFilePreviewWidth(width: number): number {
  return Math.min(FILE_PREVIEW_MAX_WIDTH, Math.max(FILE_PREVIEW_MIN_WIDTH, Math.round(width)));
}

function loadFilePreviewWidth(): number {
  return loadLayoutSize("filePreviewPanelWidth", FILE_PREVIEW_DEFAULT_WIDTH, clampFilePreviewWidth);
}

function saveFilePreviewWidth(width: number): void {
  saveLayoutSize("filePreviewPanelWidth", width, clampFilePreviewWidth);
}

function viewportWidthFallback(): number {
  if (typeof window === "undefined") return 0;
  const width = Math.round(window.innerWidth || 0);
  return Number.isFinite(width) && width > 0 ? width : 0;
}

function defaultRightDockWidth(): number {
  const width = viewportWidthFallback();
  if (width <= 0) return RIGHT_DOCK_DEFAULT_WIDTH;
  return clampRightDockWidth(width * RIGHT_DOCK_DEFAULT_RATIO);
}

function loadRightDockWidth(): number {
  const unified = loadOptionalLayoutSize("rightDockWidth", clampRightDockWidth);
  if (unified !== null) return unified;
  const preview = loadOptionalLayoutSize("rightDockPreviewWidth", clampRightDockWidth);
  const tree = loadOptionalLayoutSize("rightDockTreeWidth", clampRightDockWidth);
  if (preview !== null && tree !== null) return clampRightDockWidth(Math.max(preview, tree));
  if (preview !== null) return preview;
  if (tree !== null) return tree;
  return defaultRightDockWidth();
}

function resolveRightDockWidth(mainWidth: number, desiredDockWidth: number, maxWidth = RIGHT_DOCK_MAX_WIDTH): number {
  const budget = Math.max(0, Math.round(mainWidth) - DOCK_CHAT_MIN_WIDTH - WORKSPACE_RESIZER_WIDTH);
  if (budget < RIGHT_DOCK_MIN_RENDER_WIDTH) return 0;
  const desired = Math.min(maxWidth, Math.max(RIGHT_DOCK_MIN_RENDER_WIDTH, Math.round(desiredDockWidth)));
  return Math.min(budget, desired);
}

/** Half of the full workbench width, leaving minimum chat space in the body row. */
function resolveExpandedPreviewDockWidth(workbenchWidth: number, bodyRowWidth: number): number {
  const body = Math.max(0, Math.round(bodyRowWidth));
  const maxAllowed = Math.max(
    RIGHT_DOCK_MIN_RENDER_WIDTH,
    body - DOCK_CHAT_MIN_WIDTH - WORKSPACE_RESIZER_WIDTH,
  );
  if (maxAllowed < RIGHT_DOCK_MIN_RENDER_WIDTH) return RIGHT_DOCK_MIN_RENDER_WIDTH;
  const bench = Math.max(0, Math.round(workbenchWidth));
  const target = Math.round(bench * PREVIEW_DOCK_EXPANDED_RATIO);
  return Math.max(RIGHT_DOCK_MIN_RENDER_WIDTH, Math.min(maxAllowed, target));
}

export type WorkbenchDockDeps = {
  appMode: AppMode;
  projectDrawerOpen: boolean;
  browserActive: boolean;
  openBrowserTab: (url?: string) => string;
  closeAllBrowserTabs: () => void;
  terminalHasSessions: boolean;
  terminalPanelVisible: boolean;
  minimizeTerminalPanel: () => void;
  restoreTerminalPanel: () => void;
  closeTerminalPanel: () => void;
  openNewTerminal: () => void | Promise<void>;
  cwd?: string;
  setComposerInsertRequest: Dispatch<SetStateAction<ComposerInsertRequest | null>>;
};

export function useWorkbenchDock(deps: WorkbenchDockDeps) {
  const {
    appMode,
    projectDrawerOpen,
    browserActive,
    openBrowserTab,
    terminalHasSessions,
    terminalPanelVisible,
    minimizeTerminalPanel,
    restoreTerminalPanel,
    openNewTerminal,
    cwd,
    setComposerInsertRequest,
  } = deps;

  const layoutRef = useRef<HTMLDivElement>(null);
  const [layoutWidth, setLayoutWidth] = useState(0);
  const [workspacePanelOpen, setWorkspacePanelOpen] = useState(false);
  const [dockAnimWidth, setDockAnimWidth] = useState(0);
  const [dockMotionKey, setDockMotionKey] = useState(0);
  const [dockClosing, setDockClosing] = useState(false);
  const dockCloseTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const targetDockWidthRef = useRef(0);
  const [rightDockWidth, setRightDockWidth] = useState(loadRightDockWidth);
  const [dockResizing, setDockResizing] = useState(false);
  const [filePreviewPath, setFilePreviewPath] = useState<string | null>(null);
  const [filePreviewDiff, setFilePreviewDiff] = useState<ToolFileDiff | null>(null);
  const [filePreviewWidth, setFilePreviewWidth] = useState(loadFilePreviewWidth);
  const [filePreviewExpanded, setFilePreviewExpanded] = useState(false);
  const [filePreviewComposerOpen, setFilePreviewComposerOpen] = useState(false);
  const [filePreviewResizing, setFilePreviewResizing] = useState(false);
  const [browserPreviewExpanded, setBrowserPreviewExpanded] = useState(false);
  const [pagePreviewPath, setPagePreviewPath] = useState<string | null>(null);
  const [rightDockMode, setRightDockMode] = useState<RightDockTab>(() => loadHubLastTab("context"));

  const chatMode = appMode === "code";
  const showRightDock = chatMode || appMode === "write";
  const preferredWorkspacePanelWidth = rightDockWidth;
  const filePreviewOpen = filePreviewPath !== null;
  const sidebarRenderWidth = projectDrawerOpen ? STUDIO_RAIL_WIDTH + STUDIO_DRAWER_WIDTH : STUDIO_RAIL_WIDTH;
  const measuredMainWidth =
    layoutWidth > 0
      ? Math.max(0, layoutWidth - sidebarRenderWidth)
      : CHAT_MIN_WIDTH + WORKSPACE_RESIZER_WIDTH + preferredWorkspacePanelWidth;
  const studioToolRailWidth = chatMode ? 52 : 0;
  const filePreviewRenderWidth = filePreviewOpen
    ? clampFilePreviewWidth(
        filePreviewExpanded
          ? Math.min(
              FILE_PREVIEW_MAX_WIDTH,
              Math.max(
                filePreviewWidth > 0 ? filePreviewWidth : preferredWorkspacePanelWidth,
                Math.round((measuredMainWidth - studioToolRailWidth) * 0.4),
              ),
            )
          : filePreviewWidth > 0
            ? filePreviewWidth
            : preferredWorkspacePanelWidth,
      )
    : 0;
  const previewChromeWidth = filePreviewOpen ? filePreviewRenderWidth + WORKSPACE_RESIZER_WIDTH : 0;
  const chatDockBudget = Math.max(0, measuredMainWidth - studioToolRailWidth - previewChromeWidth);
  const workbenchWidth = layoutWidth > 0 ? layoutWidth : viewportWidthFallback();
  const previewDockTab = rightDockMode === "browser" || rightDockMode === "page";
  const previewDockPreferred = clampRightDockWidth(
    Math.max(
      PREVIEW_DOCK_MIN_WIDTH,
      Math.round(Math.max(workbenchWidth, chatDockBudget) * PREVIEW_DOCK_DEFAULT_RATIO),
    ),
  );
  const dockPreferredWidth =
    workspacePanelOpen && previewDockTab
      ? Math.max(preferredWorkspacePanelWidth, previewDockPreferred)
      : preferredWorkspacePanelWidth;
  const resolvedWorkspacePanelWidth = workspacePanelOpen
    ? resolveRightDockWidth(chatDockBudget, dockPreferredWidth)
    : dockPreferredWidth;
  const baseWorkspacePanelWidth = workspacePanelOpen
    ? Math.max(resolvedWorkspacePanelWidth, RIGHT_DOCK_MIN_RENDER_WIDTH)
    : 0;
  const expandedPreviewWidth = resolveExpandedPreviewDockWidth(workbenchWidth, chatDockBudget);
  const workspacePanelRenderWidth =
    workspacePanelOpen && browserPreviewExpanded && previewDockTab
      ? expandedPreviewWidth
      : baseWorkspacePanelWidth;
  targetDockWidthRef.current = workspacePanelRenderWidth;
  const dockGridWidth = 0;
  const browserPreviewActive = browserActive;
  const browserPreviewVisible = workspacePanelOpen && rightDockMode === "browser";
  const pagePreviewActive = pagePreviewPath !== null;
  const pagePreviewVisible = workspacePanelOpen && rightDockMode === "page";
  const previewPanelVisible = browserPreviewVisible || pagePreviewVisible;
  const previewPanelActive = browserPreviewActive || pagePreviewActive;
  const dockBackgroundSessions = browserPreviewActive || pagePreviewActive;
  const dockMounted = workspacePanelOpen || dockBackgroundSessions;

  useEffect(() => {
    if (!workspacePanelOpen) {
      return;
    }
    if (dockCloseTimerRef.current) {
      clearTimeout(dockCloseTimerRef.current);
      dockCloseTimerRef.current = null;
    }
    setDockClosing(false);
    setDockMotionKey((key) => key + 1);
    setDockAnimWidth(0);
    const id = window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        setDockAnimWidth(targetDockWidthRef.current);
      });
    });
    return () => window.cancelAnimationFrame(id);
  }, [workspacePanelOpen]);

  useEffect(() => {
    if (!workspacePanelOpen || dockAnimWidth <= 0) return;
    setDockAnimWidth(workspacePanelRenderWidth);
  }, [workspacePanelRenderWidth, workspacePanelOpen, dockAnimWidth, browserPreviewExpanded]);

  useEffect(() => {
    savePreviewPanelState({
      terminal: terminalHasSessions,
      browser: browserPreviewActive,
      page: pagePreviewActive,
    });
  }, [browserPreviewActive, pagePreviewActive, terminalHasSessions]);

  useEffect(() => {
    setFilePreviewPath(null);
  }, [cwd]);

  useEffect(() => {
    const el = layoutRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const update = () => {
      const width = el.getBoundingClientRect().width;
      if (width && Number.isFinite(width)) setLayoutWidth(Math.round(width));
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const closeWorkspacePanel = useCallback(() => {
    if (!workspacePanelOpen) {
      return;
    }
    setDockClosing(true);
    setDockAnimWidth(0);
    setBrowserPreviewExpanded(false);
    if (dockCloseTimerRef.current) clearTimeout(dockCloseTimerRef.current);
    dockCloseTimerRef.current = setTimeout(() => {
      setWorkspacePanelOpen(false);
      setDockClosing(false);
      setFilePreviewPath(null);
      dockCloseTimerRef.current = null;
    }, MOTION_PANEL_MS);
  }, [workspacePanelOpen]);

  const toggleBrowserPreviewExpanded = useCallback(() => {
    setBrowserPreviewExpanded((value) => !value);
  }, []);

  const openDockTab = useCallback(
    (tab: RightDockTab, options?: { toggle?: boolean }) => {
      saveDockTabSelection(tab);
      const shouldToggle = options?.toggle !== false;
      if (shouldToggle && workspacePanelOpen && rightDockMode === tab) {
        closeWorkspacePanel();
        return;
      }
      if (tab !== "browser" && tab !== "page") {
        setBrowserPreviewExpanded(false);
      }
      setRightDockMode(tab);
      setWorkspacePanelOpen(true);
    },
    [closeWorkspacePanel, rightDockMode, workspacePanelOpen],
  );

  const openFilePreview = useCallback(
    (path: string, dockTab: RightDockTab = "files") => {
      if (isPreviewablePagePath(path) && dockTab === "files") {
        setPagePreviewPath(path);
        openDockTab("page", { toggle: false });
        return;
      }
      const width = clampFilePreviewWidth(rightDockWidth);
      setFilePreviewWidth(width);
      saveFilePreviewWidth(width);
      setFilePreviewExpanded(false);
      setFilePreviewComposerOpen(false);
      setFilePreviewPath(path);
      setFilePreviewDiff(null);
      openDockTab(dockTab, { toggle: false });
    },
    [openDockTab, rightDockWidth],
  );

  const openActionFilePreview = useCallback(
    (req: ActionFileOpenRequest) => {
      const width = clampFilePreviewWidth(rightDockWidth);
      setFilePreviewWidth(width);
      saveFilePreviewWidth(width);
      setFilePreviewExpanded(false);
      setFilePreviewComposerOpen(false);
      setFilePreviewPath(req.path);
      setFilePreviewDiff(req.diff ?? null);
    },
    [rightDockWidth],
  );

  const closeFilePreview = useCallback(() => {
    setFilePreviewPath(null);
    setFilePreviewDiff(null);
    setFilePreviewExpanded(false);
    setFilePreviewComposerOpen(false);
  }, []);

  const exitExpandedPreviewComposer = useCallback(() => {
    setFilePreviewComposerOpen(false);
    setFilePreviewExpanded(false);
  }, []);

  const toggleFilePreviewExpanded = useCallback(() => {
    setFilePreviewExpanded((value) => {
      if (value) setFilePreviewComposerOpen(false);
      return !value;
    });
  }, []);

  const togglePreviewTerminal = useCallback(() => {
    if (terminalPanelVisible) {
      minimizeTerminalPanel();
      return;
    }
    if (terminalHasSessions) {
      restoreTerminalPanel();
      return;
    }
    void openNewTerminal();
  }, [minimizeTerminalPanel, openNewTerminal, restoreTerminalPanel, terminalHasSessions, terminalPanelVisible]);

  const openWebPreview = useCallback(
    (url?: string) => {
      openBrowserTab(url);
      openDockTab("browser", { toggle: false });
      savePreviewPanelState({ terminal: terminalHasSessions, browser: true, page: pagePreviewActive });
    },
    [openBrowserTab, openDockTab, pagePreviewActive, terminalHasSessions],
  );

  const openPagePreview = useCallback(
    (path?: string) => {
      if (path?.trim()) setPagePreviewPath(path.trim());
      openDockTab("page", { toggle: false });
      savePreviewPanelState({ terminal: terminalHasSessions, browser: browserPreviewActive, page: true });
    },
    [browserPreviewActive, openDockTab, terminalHasSessions],
  );

  const openPreviewBrowser = useCallback(() => {
    openWebPreview();
  }, [openWebPreview]);

  const togglePreviewBrowser = useCallback(() => {
    if (browserPreviewVisible) {
      closeWorkspacePanel();
      return;
    }
    if (browserPreviewActive) {
      openDockTab("browser", { toggle: false });
      return;
    }
    openWebPreview();
  }, [browserPreviewActive, browserPreviewVisible, closeWorkspacePanel, openDockTab, openWebPreview]);

  const togglePreviewPage = useCallback(() => {
    if (pagePreviewVisible) {
      closeWorkspacePanel();
      return;
    }
    if (pagePreviewActive) {
      openDockTab("page", { toggle: false });
      return;
    }
    openPagePreview();
  }, [closeWorkspacePanel, openDockTab, openPagePreview, pagePreviewActive, pagePreviewVisible]);

  const deactivatePreview = useCallback(() => {
    if (terminalPanelVisible) {
      minimizeTerminalPanel();
    }
    if (previewPanelVisible) {
      closeWorkspacePanel();
    }
  }, [closeWorkspacePanel, minimizeTerminalPanel, previewPanelVisible, terminalPanelVisible]);

  const togglePreviewMode = useCallback(
    (mode: PreviewMode) => {
      if (mode === "terminal") {
        togglePreviewTerminal();
        return;
      }
      if (mode === "page") {
        togglePreviewPage();
        return;
      }
      togglePreviewBrowser();
    },
    [togglePreviewBrowser, togglePreviewPage, togglePreviewTerminal],
  );

  const openDockHub = useCallback(
    (hub: DockHub) => {
      if (hub === "preview") {
        if (previewPanelActive || terminalHasSessions) {
          deactivatePreview();
          return;
        }
        const tab = loadPreviewHubTab();
        if (tab === "browser") {
          openWebPreview();
        } else {
          openPagePreview();
        }
        const saved = loadPreviewPanelState();
        if (saved.terminal) {
          void openNewTerminal();
        }
        return;
      }

      if (workspacePanelOpen && dockHubForTab(rightDockMode) === hub) {
        closeWorkspacePanel();
        return;
      }
      const tab = resolveHubTab(hub);
      openDockTab(tab, { toggle: false });
    },
    [
      deactivatePreview,
      closeWorkspacePanel,
      openDockTab,
      openNewTerminal,
      openPagePreview,
      openWebPreview,
      previewPanelActive,
      rightDockMode,
      terminalHasSessions,
      workspacePanelOpen,
    ],
  );

  const setSavedFilePreviewWidth = useCallback((width: number) => {
    const next = clampFilePreviewWidth(width);
    setFilePreviewWidth(next);
    saveFilePreviewWidth(next);
  }, []);

  const setSavedDockWidth = useCallback((width: number) => {
    const next = clampRightDockWidth(width);
    setRightDockWidth(next);
    saveLayoutSize("rightDockWidth", next, clampRightDockWidth);
  }, []);

  const startDockResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (!workspacePanelOpen) return;
      event.preventDefault();
      setBrowserPreviewExpanded(false);
      setDockResizing(true);
      const startX = event.clientX;
      const startWidth = rightDockWidth;
      let nextWidth = startWidth;
      const onMove = (moveEvent: PointerEvent) => {
        const delta = moveEvent.clientX - startX;
        nextWidth = clampRightDockWidth(startWidth + delta);
        setRightDockWidth(nextWidth);
      };
      const onDone = () => {
        setSavedDockWidth(nextWidth);
        setDockResizing(false);
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onDone);
        window.removeEventListener("pointercancel", onDone);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      };
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onDone);
      window.addEventListener("pointercancel", onDone);
    },
    [rightDockWidth, setSavedDockWidth, workspacePanelOpen],
  );

  const startFilePreviewResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (!filePreviewOpen) return;
      event.preventDefault();
      setFilePreviewExpanded(false);
      setFilePreviewResizing(true);
      const startX = event.clientX;
      const startWidth = filePreviewWidth;
      let nextWidth = startWidth;
      const onMove = (moveEvent: PointerEvent) => {
        const delta = moveEvent.clientX - startX;
        nextWidth = clampFilePreviewWidth(startWidth + delta);
        setFilePreviewWidth(nextWidth);
      };
      const onDone = () => {
        setSavedFilePreviewWidth(nextWidth);
        setFilePreviewResizing(false);
        window.removeEventListener("pointermove", onMove);
        window.removeEventListener("pointerup", onDone);
        window.removeEventListener("pointercancel", onDone);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      };
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
      window.addEventListener("pointermove", onMove);
      window.addEventListener("pointerup", onDone);
      window.addEventListener("pointercancel", onDone);
    },
    [filePreviewOpen, filePreviewWidth, setSavedFilePreviewWidth],
  );

  const resizeFilePreviewWithKeyboard = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>) => {
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        setFilePreviewExpanded(false);
        setSavedFilePreviewWidth(filePreviewWidth + (event.key === "ArrowRight" ? 16 : -16));
      } else if (event.key === "Home") {
        event.preventDefault();
        setFilePreviewExpanded(false);
        setSavedFilePreviewWidth(FILE_PREVIEW_MIN_WIDTH);
      } else if (event.key === "End") {
        event.preventDefault();
        setFilePreviewExpanded(false);
        setSavedFilePreviewWidth(FILE_PREVIEW_MAX_WIDTH);
      }
    },
    [filePreviewWidth, setSavedFilePreviewWidth],
  );

  const resetFilePreviewWidthFromDock = useCallback(() => {
    setSavedFilePreviewWidth(clampFilePreviewWidth(rightDockWidth));
  }, [rightDockWidth, setSavedFilePreviewWidth]);

  const clearFilePreviewComposerOpen = useCallback(() => {
    setFilePreviewComposerOpen(false);
  }, []);

  const layoutStyle = useMemo(
    () =>
      ({
        "--sidebar-render-width": `${sidebarRenderWidth}px`,
        "--file-preview-render-width": `${filePreviewRenderWidth}px`,
        "--dock-render-width": `${dockGridWidth}px`,
      }) as CSSProperties,
    [dockGridWidth, filePreviewRenderWidth, sidebarRenderWidth, projectDrawerOpen],
  );

  const addWorkspaceTextToComposer = useCallback(
    (text: string, replace = false) => {
      setComposerInsertRequest({ id: Date.now(), text, replace: replace || undefined });
      if (filePreviewExpanded) {
        setFilePreviewComposerOpen(true);
        return;
      }
      setFilePreviewComposerOpen(false);
    },
    [filePreviewExpanded, setComposerInsertRequest],
  );

  return {
    layoutRef,
    layoutStyle,
    workspacePanelOpen,
    dockAnimWidth,
    dockMotionKey,
    dockClosing,
    dockResizing,
    filePreviewPath,
    filePreviewDiff,
    filePreviewExpanded,
    filePreviewComposerOpen,
    filePreviewResizing,
    rightDockMode,
    rightDockWidth,
    browserPreviewExpanded,
    browserPreviewActive,
    browserPreviewVisible,
    pagePreviewActive,
    pagePreviewVisible,
    previewPanelVisible,
    previewPanelActive,
    dockMounted,
    dockBackgroundSessions,
    openWebPreview,
    openPagePreview,
    openPreviewBrowser,
    filePreviewOpen,
    pagePreviewPath,
    setPagePreviewPath,
    showRightDock,
    closeWorkspacePanel,
    openDockTab,
    openFilePreview,
    openActionFilePreview,
    closeFilePreview,
    exitExpandedPreviewComposer,
    toggleFilePreviewExpanded,
    toggleBrowserPreviewExpanded,
    togglePreviewTerminal,
    togglePreviewMode,
    openDockHub,
    startDockResize,
    startFilePreviewResize,
    resizeFilePreviewWithKeyboard,
    resetFilePreviewWidthFromDock,
    clearFilePreviewComposerOpen,
    addWorkspaceTextToComposer,
  };
}
