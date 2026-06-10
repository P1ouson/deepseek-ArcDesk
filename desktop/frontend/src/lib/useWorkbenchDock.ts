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
const RIGHT_DOCK_DEFAULT_WIDTH = 300;
const RIGHT_DOCK_DEFAULT_RATIO = 0.2;
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

function resolveRightDockWidth(mainWidth: number, desiredDockWidth: number): number {
  const budget = Math.max(0, Math.round(mainWidth) - DOCK_CHAT_MIN_WIDTH - WORKSPACE_RESIZER_WIDTH);
  if (budget < RIGHT_DOCK_MIN_RENDER_WIDTH) return 0;
  const desired = Math.min(RIGHT_DOCK_MAX_WIDTH, Math.max(RIGHT_DOCK_MIN_RENDER_WIDTH, Math.round(desiredDockWidth)));
  return Math.min(budget, desired);
}

export type WorkbenchDockDeps = {
  appMode: AppMode;
  projectDrawerOpen: boolean;
  terminalOpen: boolean;
  cwd?: string;
  openNewTerminal: () => void | Promise<void>;
  closeTerminalPanel: () => void;
  setComposerInsertRequest: Dispatch<SetStateAction<ComposerInsertRequest | null>>;
};

export function useWorkbenchDock(deps: WorkbenchDockDeps) {
  const { appMode, projectDrawerOpen, terminalOpen, cwd, openNewTerminal, closeTerminalPanel, setComposerInsertRequest } =
    deps;

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
  const [webPreviewUrl, setWebPreviewUrl] = useState<string | null>(null);
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
  const resolvedWorkspacePanelWidth = workspacePanelOpen
    ? resolveRightDockWidth(chatDockBudget, preferredWorkspacePanelWidth)
    : preferredWorkspacePanelWidth;
  const baseWorkspacePanelWidth = workspacePanelOpen
    ? Math.max(resolvedWorkspacePanelWidth, RIGHT_DOCK_MIN_RENDER_WIDTH)
    : 0;
  const workspacePanelRenderWidth =
    workspacePanelOpen && browserPreviewExpanded
      ? clampRightDockWidth(Math.min(RIGHT_DOCK_MAX_WIDTH, Math.max(baseWorkspacePanelWidth, 480)))
      : baseWorkspacePanelWidth;
  targetDockWidthRef.current = workspacePanelRenderWidth;
  const dockGridWidth = 0;
  const browserPreviewOpen = workspacePanelOpen && rightDockMode === "browser";
  const pagePreviewOpen = workspacePanelOpen && rightDockMode === "page";
  const previewPanelOpen = browserPreviewOpen || pagePreviewOpen;

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
  }, [workspacePanelRenderWidth, workspacePanelOpen, dockAnimWidth]);

  useEffect(() => {
    savePreviewPanelState({ terminal: terminalOpen, browser: browserPreviewOpen, page: pagePreviewOpen });
  }, [browserPreviewOpen, pagePreviewOpen, terminalOpen]);

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

  const closePreviewPanel = useCallback(() => {
    setBrowserPreviewExpanded(false);
    if (previewPanelOpen) {
      closeWorkspacePanel();
    }
  }, [closeWorkspacePanel, previewPanelOpen]);

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
    if (terminalOpen) {
      closeTerminalPanel();
      return;
    }
    void openNewTerminal();
  }, [closeTerminalPanel, openNewTerminal, terminalOpen]);

  const openWebPreview = useCallback(
    (url?: string) => {
      if (url?.trim()) setWebPreviewUrl(url.trim());
      openDockTab("browser", { toggle: false });
      savePreviewPanelState({ terminal: terminalOpen, browser: true, page: false });
    },
    [openDockTab, terminalOpen],
  );

  const openPagePreview = useCallback(
    (path?: string) => {
      if (path?.trim()) setPagePreviewPath(path.trim());
      openDockTab("page", { toggle: false });
      savePreviewPanelState({ terminal: terminalOpen, browser: false, page: true });
    },
    [openDockTab, terminalOpen],
  );

  const openPreviewBrowser = useCallback(() => {
    openWebPreview();
  }, [openWebPreview]);

  const togglePreviewBrowser = useCallback(() => {
    if (browserPreviewOpen) {
      closePreviewPanel();
      return;
    }
    openWebPreview();
  }, [browserPreviewOpen, closePreviewPanel, openWebPreview]);

  const togglePreviewPage = useCallback(() => {
    if (pagePreviewOpen) {
      closePreviewPanel();
      return;
    }
    openPagePreview();
  }, [closePreviewPanel, openPagePreview, pagePreviewOpen]);

  const deactivatePreview = useCallback(() => {
    if (terminalOpen) {
      closeTerminalPanel();
    }
    if (previewPanelOpen) {
      closePreviewPanel();
    }
  }, [closePreviewPanel, closeTerminalPanel, previewPanelOpen, terminalOpen]);

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
        if (previewPanelOpen || terminalOpen) {
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
      previewPanelOpen,
      rightDockMode,
      terminalOpen,
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
    browserPreviewOpen,
    pagePreviewOpen,
    previewPanelOpen,
    webPreviewUrl,
    setWebPreviewUrl,
    pagePreviewPath,
    setPagePreviewPath,
    openWebPreview,
    openPagePreview,
    openPreviewBrowser,
    filePreviewOpen,
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
