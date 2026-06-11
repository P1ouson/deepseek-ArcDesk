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
import { attachPointerResize } from "./panelResize";
import type { ToolFileDiff } from "./tools";
import type { ComposerInsertRequest } from "./types";
import { isPreviewablePagePath } from "./previewPage";
import {
  commitDockResize,
  commitPreviewResize,
  maxDockPanelWidth,
  maxFilePreviewWidth,
  PANEL_RESIZER_WIDTH,
  workbenchRowWidth,
  type WorkbenchRowChrome,
} from "./workbenchPanelLayout";

const STUDIO_RAIL_WIDTH = 76;
const STUDIO_DRAWER_WIDTH = 280;
const CHAT_MIN_WIDTH = 760;
/** Panel slide duration — keep in sync with --duration-normal in design-system.css */
const MOTION_PANEL_MS = 220;

const RIGHT_DOCK_DEFAULT_WIDTH = 380;
const RIGHT_DOCK_DEFAULT_RATIO = 0.28;
const PREVIEW_DOCK_EXPANDED_RATIO = 0.5;
const RIGHT_DOCK_MIN_WIDTH = 280;
const RIGHT_DOCK_MAX_WIDTH = 800;
const RIGHT_DOCK_MIN_RENDER_WIDTH = 200;
const FILE_PREVIEW_MIN_WIDTH = RIGHT_DOCK_MIN_WIDTH;
const FILE_PREVIEW_MAX_WIDTH = 960;
/** Code preview default — ~40% of main row width (see red-line layout target in studio). */
const FILE_PREVIEW_DEFAULT_WIDTH = 620;
const FILE_PREVIEW_DEFAULT_RATIO = 0.4;
const FILE_PREVIEW_EXPANDED_RATIO = 0.58;

function clampRightDockWidth(width: number): number {
  return Math.min(RIGHT_DOCK_MAX_WIDTH, Math.max(RIGHT_DOCK_MIN_WIDTH, Math.round(width)));
}

function clampFilePreviewWidth(width: number): number {
  return Math.min(FILE_PREVIEW_MAX_WIDTH, Math.max(FILE_PREVIEW_MIN_WIDTH, Math.round(width)));
}

function defaultFilePreviewWidth(): number {
  const width = viewportWidthFallback();
  if (width <= 0) return FILE_PREVIEW_DEFAULT_WIDTH;
  const main = Math.max(0, width - STUDIO_RAIL_WIDTH);
  return clampFilePreviewWidth(Math.max(FILE_PREVIEW_DEFAULT_WIDTH, Math.round(main * FILE_PREVIEW_DEFAULT_RATIO)));
}

function loadFilePreviewWidth(): number {
  const width = loadLayoutSize("filePreviewPanelWidth", FILE_PREVIEW_DEFAULT_WIDTH, clampFilePreviewWidth);
  // Bump legacy narrow saves so reopening preview matches the wider studio default.
  if (width < 560) return defaultFilePreviewWidth();
  return width;
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

function clampDockRenderWidth(desired: number, maxAllowed: number): number {
  if (maxAllowed < RIGHT_DOCK_MIN_RENDER_WIDTH) return 0;
  return Math.min(
    RIGHT_DOCK_MAX_WIDTH,
    Math.max(RIGHT_DOCK_MIN_RENDER_WIDTH, Math.min(Math.round(desired), Math.round(maxAllowed))),
  );
}

/** Browser/page dock expanded width, capped by the workbench row budget. */
function resolveExpandedPreviewDockWidth(
  workbenchWidth: number,
  rowWidth: number,
  previewWidth: number,
  rowChrome: WorkbenchRowChrome,
): number {
  const bench = Math.max(0, Math.round(workbenchWidth));
  const target = Math.round(bench * PREVIEW_DOCK_EXPANDED_RATIO);
  const maxAllowed = maxDockPanelWidth(rowWidth, previewWidth, rowChrome);
  return clampDockRenderWidth(target, maxAllowed);
}

function fitFilePreviewWidth(
  desired: number,
  rowWidth: number,
  dockWidth: number,
  rowChrome: WorkbenchRowChrome,
): number {
  const maxAllowed = maxFilePreviewWidth(rowWidth, dockWidth, rowChrome);
  return clampFilePreviewWidth(Math.min(desired, maxAllowed));
}

function fitDockWidth(
  desired: number,
  rowWidth: number,
  previewWidth: number,
  rowChrome: WorkbenchRowChrome,
): number {
  const maxAllowed = maxDockPanelWidth(rowWidth, previewWidth, rowChrome);
  return clampDockRenderWidth(desired, maxAllowed);
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
  const [dockOpening, setDockOpening] = useState(false);
  const [dockMotionKey, setDockMotionKey] = useState(0);
  const [dockClosing, setDockClosing] = useState(false);
  const dockCloseTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const targetDockWidthRef = useRef(0);
  /** Freeze the opposite panel while dragging so freed space goes to chat, not auto-stolen. */
  const resizePartnerRef = useRef({ preview: 0, dock: 0 });
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
  const filePreviewOpen = filePreviewPath !== null;
  const sidebarRenderWidth = projectDrawerOpen ? STUDIO_RAIL_WIDTH + STUDIO_DRAWER_WIDTH : STUDIO_RAIL_WIDTH;
  const measuredMainWidth =
    layoutWidth > 0
      ? Math.max(0, layoutWidth - sidebarRenderWidth)
      : CHAT_MIN_WIDTH + PANEL_RESIZER_WIDTH + rightDockWidth;
  const workbenchWidth = layoutWidth > 0 ? layoutWidth : viewportWidthFallback();
  const rowWidth = workbenchRowWidth(measuredMainWidth, { toolRail: chatMode });
  const rowChrome: WorkbenchRowChrome = {
    previewOpen: filePreviewOpen,
    dockOpen: workspacePanelOpen,
    toolRail: chatMode,
  };
  const previewDockTab = rightDockMode === "browser" || rightDockMode === "page";
  const filePreviewRenderWidth = filePreviewOpen
    ? filePreviewExpanded
      ? fitFilePreviewWidth(
          Math.min(
            FILE_PREVIEW_MAX_WIDTH,
            Math.max(filePreviewWidth, Math.round(rowWidth * FILE_PREVIEW_EXPANDED_RATIO)),
          ),
          rowWidth,
          rightDockWidth,
          rowChrome,
        )
      : clampFilePreviewWidth(filePreviewWidth)
    : 0;
  const expandedPreviewWidth = resolveExpandedPreviewDockWidth(
    workbenchWidth,
    rowWidth,
    filePreviewRenderWidth,
    rowChrome,
  );
  const workspacePanelRenderWidth =
    workspacePanelOpen && browserPreviewExpanded && previewDockTab
      ? expandedPreviewWidth
      : workspacePanelOpen
        ? clampRightDockWidth(rightDockWidth)
        : 0;
  const dockPanelWidth = !workspacePanelOpen
    ? 0
    : dockClosing || dockOpening
      ? Math.max(0, dockAnimWidth)
      : workspacePanelRenderWidth;
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
    setDockOpening(true);
    setDockAnimWidth(0);
    const id = window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        setDockAnimWidth(targetDockWidthRef.current);
      });
    });
    const openingTimer = window.setTimeout(() => setDockOpening(false), MOTION_PANEL_MS);
    return () => {
      window.cancelAnimationFrame(id);
      window.clearTimeout(openingTimer);
    };
  }, [workspacePanelOpen]);

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
      const openingPreview = filePreviewPath === null;
      if (openingPreview) {
        const width = fitFilePreviewWidth(
          Math.max(defaultFilePreviewWidth(), filePreviewWidth),
          rowWidth,
          rightDockWidth,
          { previewOpen: true, dockOpen: true, toolRail: chatMode },
        );
        setFilePreviewWidth(width);
        saveFilePreviewWidth(width);
      }
      setFilePreviewExpanded(false);
      setFilePreviewComposerOpen(false);
      setFilePreviewPath(path);
      setFilePreviewDiff(null);
      openDockTab(dockTab, { toggle: false });
    },
    [chatMode, filePreviewPath, filePreviewWidth, openDockTab, rightDockWidth, rowWidth],
  );

  const openActionFilePreview = useCallback(
    (req: ActionFileOpenRequest) => {
      if (filePreviewPath === null) {
        const width = fitFilePreviewWidth(
          Math.max(defaultFilePreviewWidth(), filePreviewWidth),
          rowWidth,
          rightDockWidth,
          { previewOpen: true, dockOpen: workspacePanelOpen, toolRail: chatMode },
        );
        setFilePreviewWidth(width);
        saveFilePreviewWidth(width);
      }
      setFilePreviewExpanded(false);
      setFilePreviewComposerOpen(false);
      setFilePreviewPath(req.path);
      setFilePreviewDiff(req.diff ?? null);
    },
    [chatMode, filePreviewPath, filePreviewWidth, rightDockWidth, rowWidth, workspacePanelOpen],
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
    setDockAnimWidth(next);
    saveLayoutSize("rightDockWidth", next, clampRightDockWidth);
  }, []);

  const endPanelResize = useCallback((clear: () => void) => {
    window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => clear());
    });
  }, []);

  const measureRowWidth = useCallback(() => {
    const layoutWidthLive = layoutRef.current?.getBoundingClientRect().width;
    const mainWidth =
      layoutWidthLive && Number.isFinite(layoutWidthLive)
        ? Math.max(0, Math.round(layoutWidthLive) - sidebarRenderWidth)
        : measuredMainWidth;
    return workbenchRowWidth(mainWidth, { toolRail: chatMode });
  }, [chatMode, measuredMainWidth, sidebarRenderWidth]);

  const startDockResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (!workspacePanelOpen) return;
      setBrowserPreviewExpanded(false);
      const startX = event.clientX;
      const startWidth = rightDockWidth;
      resizePartnerRef.current.preview = filePreviewRenderWidth;
      resizePartnerRef.current.dock = startWidth;
      let nextWidth = startWidth;
      const rowChromeLive: WorkbenchRowChrome = {
        previewOpen: filePreviewOpen,
        dockOpen: true,
        toolRail: chatMode,
      };
      attachPointerResize({
        event,
        cursor: "col-resize",
        onStart: () => {
          setDockResizing(true);
          setRightDockWidth(startWidth);
        },
        onMove: (moveEvent) => {
          const liveRow = measureRowWidth();
          // Left edge of a right-side panel: drag left → wider, drag right → narrower.
          const delta = startX - moveEvent.clientX;
          const maxAllowed = maxDockPanelWidth(liveRow, resizePartnerRef.current.preview, rowChromeLive);
          nextWidth = clampDockRenderWidth(startWidth + delta, maxAllowed);
          setRightDockWidth(nextWidth);
        },
        onCommit: () => {
          const committed = commitDockResize(
            resizePartnerRef.current.preview,
            nextWidth,
            clampFilePreviewWidth,
            clampRightDockWidth,
          );
          setSavedFilePreviewWidth(committed.preview);
          setSavedDockWidth(committed.dock);
          endPanelResize(() => setDockResizing(false));
        },
      });
    },
    [
      chatMode,
      endPanelResize,
      filePreviewOpen,
      filePreviewRenderWidth,
      measureRowWidth,
      rightDockWidth,
      setSavedDockWidth,
      setSavedFilePreviewWidth,
      workspacePanelOpen,
    ],
  );

  const startFilePreviewResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (!filePreviewOpen) return;
      const startX = event.clientX;
      const startWidth = filePreviewRenderWidth;
      resizePartnerRef.current.preview = startWidth;
      resizePartnerRef.current.dock =
        workspacePanelRenderWidth > 0 ? workspacePanelRenderWidth : rightDockWidth;
      let nextWidth = startWidth;
      const rowChromeLive: WorkbenchRowChrome = {
        previewOpen: true,
        dockOpen: workspacePanelOpen,
        toolRail: chatMode,
      };
      attachPointerResize({
        event,
        cursor: "col-resize",
        onStart: () => {
          setFilePreviewExpanded(false);
          setFilePreviewResizing(true);
          setFilePreviewWidth(startWidth);
        },
        onMove: (moveEvent) => {
          const liveRow = measureRowWidth();
          // Left edge of preview: drag left → wider, drag right → narrower.
          const delta = startX - moveEvent.clientX;
          nextWidth = fitFilePreviewWidth(
            startWidth + delta,
            liveRow,
            resizePartnerRef.current.dock,
            rowChromeLive,
          );
          setFilePreviewWidth(nextWidth);
        },
        onCommit: () => {
          const committed = commitPreviewResize(
            nextWidth,
            resizePartnerRef.current.dock,
            clampFilePreviewWidth,
            clampRightDockWidth,
          );
          setSavedFilePreviewWidth(committed.preview);
          if (committed.dock !== rightDockWidth) {
            setSavedDockWidth(committed.dock);
          }
          endPanelResize(() => setFilePreviewResizing(false));
        },
      });
    },
    [
      chatMode,
      filePreviewOpen,
      filePreviewRenderWidth,
      measureRowWidth,
      rightDockWidth,
      endPanelResize,
      setSavedDockWidth,
      setSavedFilePreviewWidth,
      workspacePanelOpen,
      workspacePanelRenderWidth,
    ],
  );

  const resizeFilePreviewWithKeyboard = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>) => {
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        setFilePreviewExpanded(false);
        const delta = event.key === "ArrowRight" ? 16 : -16;
        setSavedFilePreviewWidth(
          fitFilePreviewWidth(filePreviewWidth + delta, rowWidth, rightDockWidth, rowChrome),
        );
      } else if (event.key === "Home") {
        event.preventDefault();
        setFilePreviewExpanded(false);
        setSavedFilePreviewWidth(FILE_PREVIEW_MIN_WIDTH);
      } else if (event.key === "End") {
        event.preventDefault();
        setFilePreviewExpanded(false);
        setSavedFilePreviewWidth(
          fitFilePreviewWidth(FILE_PREVIEW_MAX_WIDTH, rowWidth, rightDockWidth, rowChrome),
        );
      }
    },
    [filePreviewWidth, rightDockWidth, rowChrome, rowWidth, setSavedFilePreviewWidth],
  );

  const resizeDockWithKeyboard = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>) => {
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        setBrowserPreviewExpanded(false);
        const delta = event.key === "ArrowRight" ? 16 : -16;
        const next = fitDockWidth(rightDockWidth + delta, rowWidth, filePreviewRenderWidth, rowChrome);
        setSavedDockWidth(next);
      } else if (event.key === "Home") {
        event.preventDefault();
        setBrowserPreviewExpanded(false);
        const next = fitDockWidth(RIGHT_DOCK_MIN_WIDTH, rowWidth, filePreviewRenderWidth, rowChrome);
        setSavedDockWidth(next);
      } else if (event.key === "End") {
        event.preventDefault();
        setBrowserPreviewExpanded(false);
        const next = fitDockWidth(RIGHT_DOCK_MAX_WIDTH, rowWidth, filePreviewRenderWidth, rowChrome);
        setSavedDockWidth(next);
      }
    },
    [
      filePreviewRenderWidth,
      rightDockWidth,
      rowChrome,
      rowWidth,
      setSavedDockWidth,
    ],
  );

  const resetFilePreviewWidthFromDock = useCallback(() => {
    setSavedFilePreviewWidth(
      fitFilePreviewWidth(defaultFilePreviewWidth(), rowWidth, rightDockWidth, rowChrome),
    );
  }, [rightDockWidth, rowChrome, rowWidth, setSavedFilePreviewWidth]);

  const resetDockWidthFromDefault = useCallback(() => {
    const next = fitDockWidth(defaultRightDockWidth(), rowWidth, filePreviewRenderWidth, rowChrome);
    setSavedDockWidth(next);
  }, [filePreviewRenderWidth, rowChrome, rowWidth, setSavedDockWidth]);

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
    dockPanelWidth,
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
    resizeDockWithKeyboard,
    resetFilePreviewWidthFromDock,
    resetDockWidthFromDefault,
    clearFilePreviewComposerOpen,
    addWorkspaceTextToComposer,
  };
}
