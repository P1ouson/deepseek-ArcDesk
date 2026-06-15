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
import type { SidebarPrimaryTab } from "./sidebarViews";
import { isSidebarPanelTabAllowed } from "./sidebarViews";
import { shouldOpenNewBrowserTab } from "./previewBrowserTabPolicy";
import type { ActionFileOpenRequest } from "../components/ActionStream";
import type { AppMode } from "./appMode";
import {
  dockHubForTab,
  loadHubLastTab,
  loadPreviewPanelState,
  resolveHubTab,
  saveDockTabSelection,
  savePreviewPanelState,
  type DockHub,
  type PreviewMode,
} from "./dockHubs";
import { loadOptionalLayoutSize, saveLayoutSize } from "./layoutPreferences";
import { attachPointerResize } from "./panelResize";
import type { ToolFileDiff } from "./tools";
import type { ComposerInsertRequest } from "./types";
import { isPreviewablePagePath } from "./previewPage";
import {
  commitDockResize,
  commitPreviewResize,
  fitStudioRightPanels,
  maxStudioDockWidth,
  maxStudioPreviewWidth,
  STUDIO_FILE_TREE_SPLIT,
  studioDrawerWidth,
  studioFileTreeSplitWidths,
  studioSinglePanelTargetWidth,
  type StudioLayoutProfile,
  type WorkbenchRowChrome,
} from "./workbenchPanelLayout";

const STUDIO_RAIL_WIDTH = 76;
import { MOTION_DURATION_NORMAL_MS } from "./motion/constants";

const RIGHT_DOCK_DEFAULT_WIDTH = 380;
const RIGHT_DOCK_MIN_WIDTH = 280;
const RIGHT_DOCK_MAX_WIDTH = 800;
const RIGHT_DOCK_MIN_RENDER_WIDTH = 200;
const FILE_PREVIEW_MIN_WIDTH = RIGHT_DOCK_MIN_WIDTH;
const FILE_PREVIEW_MAX_WIDTH = 960;
const FILE_PREVIEW_DEFAULT_WIDTH = 620;

const STUDIO_PANEL_BOUNDS = {
  previewMin: FILE_PREVIEW_MIN_WIDTH,
  previewMax: FILE_PREVIEW_MAX_WIDTH,
  dockMin: RIGHT_DOCK_MIN_WIDTH,
  dockMax: RIGHT_DOCK_MAX_WIDTH,
} as const;

function clampRightDockWidth(width: number): number {
  return Math.min(RIGHT_DOCK_MAX_WIDTH, Math.max(RIGHT_DOCK_MIN_WIDTH, Math.round(width)));
}

function clampFilePreviewWidth(width: number): number {
  return Math.min(FILE_PREVIEW_MAX_WIDTH, Math.max(FILE_PREVIEW_MIN_WIDTH, Math.round(width)));
}

function defaultFilePreviewWidth(layoutWidth = viewportWidthFallback()): number {
  if (layoutWidth <= 0) return FILE_PREVIEW_DEFAULT_WIDTH;
  return clampFilePreviewWidth(studioSinglePanelTargetWidth(layoutWidth, true));
}

function loadFilePreviewWidth(): number {
  const saved = loadOptionalLayoutSize("filePreviewPanelWidth", clampFilePreviewWidth);
  if (saved !== null) return saved;
  return defaultFilePreviewWidth();
}

function saveFilePreviewWidth(width: number): void {
  saveLayoutSize("filePreviewPanelWidth", width, clampFilePreviewWidth);
}

function viewportWidthFallback(): number {
  if (typeof window === "undefined") return 0;
  const width = Math.round(window.innerWidth || 0);
  return Number.isFinite(width) && width > 0 ? width : 0;
}

function defaultRightDockWidth(layoutWidth = viewportWidthFallback()): number {
  if (layoutWidth <= 0) return RIGHT_DOCK_DEFAULT_WIDTH;
  return clampRightDockWidth(studioSinglePanelTargetWidth(layoutWidth, true));
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

/** Browser/page dock expanded width, capped by the studio right budget. */
function fitFilePreviewWidth(
  desired: number,
  layoutWidth: number,
  dockWidth: number,
  rowChrome: WorkbenchRowChrome,
  profile: StudioLayoutProfile = "code",
): number {
  const maxAllowed = maxStudioPreviewWidth(layoutWidth, dockWidth, rowChrome, profile);
  return clampFilePreviewWidth(Math.min(desired, maxAllowed));
}

function fitDockWidth(
  desired: number,
  layoutWidth: number,
  previewWidth: number,
  rowChrome: WorkbenchRowChrome,
  profile: StudioLayoutProfile = "code",
): number {
  const maxAllowed = maxStudioDockWidth(layoutWidth, previewWidth, rowChrome, profile);
  return clampDockRenderWidth(desired, maxAllowed);
}

function rightDockTabToSidebar(tab: RightDockTab): SidebarPrimaryTab {
  if (tab === "git") return "git";
  if (tab === "context") return "context";
  if (tab === "todo") return "todo";
  if (tab === "changes") return "changes";
  return "files";
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
  setActiveBrowserTabId: (id: string) => void;
  setActiveTerminalId: (id: string) => void;
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
    minimizeTerminalPanel,
    restoreTerminalPanel,
    openNewTerminal,
    setActiveBrowserTabId,
    setActiveTerminalId,
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
  const previewWidthUserSizedRef = useRef(false);
  const dockWidthUserSizedRef = useRef(false);
  const [fileTreeLayoutUserSized, setFileTreeLayoutUserSized] = useState(false);
  const [fileTreePreviewExpanded, setFileTreePreviewExpanded] = useState(false);
  const skipDockOpenAnimationRef = useRef(false);
  const [rightDockWidth, setRightDockWidth] = useState(loadRightDockWidth);
  const [dockResizing, setDockResizing] = useState(false);
  const [filePreviewPath, setFilePreviewPath] = useState<string | null>(null);
  const [filePreviewDiff, setFilePreviewDiff] = useState<ToolFileDiff | null>(null);
  const [filePreviewWidth, setFilePreviewWidth] = useState(loadFilePreviewWidth);
  const [previewColumnExpanded, setPreviewColumnExpanded] = useState(false);
  const [previewMode, setPreviewMode] = useState<PreviewMode>(() => {
    const saved = loadPreviewPanelState();
    if (saved.file) return "file";
    if (saved.browser) return "browser";
    if (saved.terminal) return "terminal";
    return "page";
  });
  const [previewColumnOpen, setPreviewColumnOpen] = useState(false);
  const [previewTerminalDocked, setPreviewTerminalDocked] = useState(false);
  const [filePreviewComposerOpen, setFilePreviewComposerOpen] = useState(false);
  const [filePreviewResizing, setFilePreviewResizing] = useState(false);
  const [pagePreviewPath, setPagePreviewPath] = useState<string | null>(null);
  const [rightDockMode, setRightDockMode] = useState<RightDockTab>(() => loadHubLastTab("context"));
  const [sidebarBodyTab, setSidebarBodyTab] = useState<SidebarPrimaryTab>(() =>
    rightDockTabToSidebar(loadHubLastTab("context")),
  );
  const [lastPanelTab, setLastPanelTab] = useState<SidebarPrimaryTab>(() =>
    rightDockTabToSidebar(loadHubLastTab("context")),
  );

  const chatMode = appMode === "code";
  const layoutProfile: StudioLayoutProfile = appMode === "write" ? "write" : "code";
  const showRightDock = chatMode || appMode === "write";
  const layoutWidthLive = layoutWidth > 0 ? layoutWidth : viewportWidthFallback();
  const studioDrawerRenderWidth = studioDrawerWidth(layoutWidthLive, STUDIO_RAIL_WIDTH, projectDrawerOpen, layoutProfile);
  const sidebarRenderWidth = STUDIO_RAIL_WIDTH + studioDrawerRenderWidth;
  const browserPreviewActive = browserActive;
  const pagePreviewActive = pagePreviewPath !== null;
  const filePreviewOpen = filePreviewPath !== null;
  const previewColumnActive =
    previewColumnOpen &&
    (filePreviewOpen || pagePreviewActive || browserPreviewActive || (previewTerminalDocked && terminalHasSessions));
  const sidebarOpen = workspacePanelOpen || previewColumnOpen;
  const fileTreePreviewContext =
    previewColumnActive &&
    workspacePanelOpen &&
    (filePreviewOpen || (pagePreviewActive && previewMode === "page"));
  const fileTreeSplitActive = false;
  const studioRightChrome: WorkbenchRowChrome = {
    previewOpen: false,
    dockOpen: sidebarOpen && !fileTreePreviewExpanded,
    toolRail: false,
  };
  const fittedRightPanels = fitStudioRightPanels(
    filePreviewWidth,
    rightDockWidth,
    layoutWidthLive,
    studioRightChrome,
    STUDIO_PANEL_BOUNDS,
    {
      previewUserSized: fileTreePreviewExpanded
        ? false
        : previewWidthUserSizedRef.current,
      dockUserSized: fileTreePreviewExpanded ? false : dockWidthUserSizedRef.current,
      splitRatio:
        fileTreeSplitActive && !fileTreeLayoutUserSized
          ? STUDIO_FILE_TREE_SPLIT
          : undefined,
    },
    layoutProfile,
  );
  const previewColumnRenderWidth = 0;
  const filePreviewRenderWidth = 0;
  const rowChrome: WorkbenchRowChrome = studioRightChrome;
  const sidebarWidthTarget = fileTreePreviewExpanded
    ? clampRightDockWidth(studioSinglePanelTargetWidth(layoutWidthLive, chatMode, layoutProfile))
    : fittedRightPanels.dock > 0
      ? fittedRightPanels.dock
      : fittedRightPanels.preview;
  const workspacePanelRenderWidth = sidebarOpen ? sidebarWidthTarget : 0;
  const dockPanelWidth = !sidebarOpen
    ? 0
    : dockClosing || dockOpening
      ? Math.max(0, dockAnimWidth)
      : dockResizing
        ? clampRightDockWidth(rightDockWidth)
        : workspacePanelRenderWidth;
  targetDockWidthRef.current = workspacePanelRenderWidth;
  const dockGridWidth = 0;
  const browserPreviewVisible = previewColumnActive && previewMode === "browser";
  const pagePreviewVisible = previewColumnActive && previewMode === "page";
  const previewPanelVisible = previewColumnActive;
  const previewPanelActive =
    browserPreviewActive || pagePreviewActive || filePreviewOpen || (previewTerminalDocked && terminalHasSessions);
  const dockBackgroundSessions = previewPanelActive && !previewColumnOpen;
  const dockMounted = sidebarOpen || dockBackgroundSessions;

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
    if (skipDockOpenAnimationRef.current) {
      skipDockOpenAnimationRef.current = false;
      setDockOpening(false);
      setDockAnimWidth(targetDockWidthRef.current);
      return;
    }
    setDockOpening(true);
    setDockAnimWidth(0);
    const id = window.requestAnimationFrame(() => {
      window.requestAnimationFrame(() => {
        setDockAnimWidth(targetDockWidthRef.current);
      });
    });
    const openingTimer = window.setTimeout(() => setDockOpening(false), MOTION_DURATION_NORMAL_MS);
    return () => {
      window.cancelAnimationFrame(id);
      window.clearTimeout(openingTimer);
    };
  }, [workspacePanelOpen]);

  useEffect(() => {
    savePreviewPanelState({
      terminal: previewTerminalDocked && terminalHasSessions,
      browser: browserPreviewActive,
      page: pagePreviewActive,
      file: filePreviewOpen,
    });
  }, [browserPreviewActive, filePreviewOpen, pagePreviewActive, previewTerminalDocked, terminalHasSessions]);

  const applyStudioRightPanelDefault = useCallback(
    (kind: "preview" | "dock") => {
      const target = studioSinglePanelTargetWidth(layoutWidthLive, chatMode, layoutProfile);
      if (kind === "preview") {
        previewWidthUserSizedRef.current = false;
        setFilePreviewWidth(target);
        saveFilePreviewWidth(target);
        return;
      }
      dockWidthUserSizedRef.current = false;
      setRightDockWidth(target);
      setDockAnimWidth(target);
      saveLayoutSize("rightDockWidth", target, clampRightDockWidth);
    },
    [chatMode, layoutProfile, layoutWidthLive],
  );

  useEffect(() => {
    if (!layoutWidthLive) return;
    if (fileTreePreviewExpanded) {
      const target = studioSinglePanelTargetWidth(layoutWidthLive, chatMode, layoutProfile);
      if (filePreviewWidth !== target) {
        setFilePreviewWidth(target);
        saveFilePreviewWidth(target);
      }
      return;
    }
    if (fileTreeSplitActive && !fileTreeLayoutUserSized) {
      const split = studioFileTreeSplitWidths(layoutWidthLive, chatMode, layoutProfile);
      if (filePreviewWidth !== split.preview) {
        setFilePreviewWidth(split.preview);
        saveFilePreviewWidth(split.preview);
      }
      if (rightDockWidth !== split.dock) {
        setRightDockWidth(split.dock);
        setDockAnimWidth(split.dock);
        saveLayoutSize("rightDockWidth", split.dock, clampRightDockWidth);
      }
      return;
    }
    if (previewColumnActive && !workspacePanelOpen && !previewWidthUserSizedRef.current) {
      const target = studioSinglePanelTargetWidth(layoutWidthLive, chatMode, layoutProfile);
      if (filePreviewWidth !== target) {
        setFilePreviewWidth(target);
        saveFilePreviewWidth(target);
      }
    }
    if (workspacePanelOpen && !previewColumnActive && !dockWidthUserSizedRef.current) {
      const target = studioSinglePanelTargetWidth(layoutWidthLive, chatMode, layoutProfile);
      if (rightDockWidth !== target) {
        setRightDockWidth(target);
        setDockAnimWidth(target);
        saveLayoutSize("rightDockWidth", target, clampRightDockWidth);
      }
    }
  }, [
    chatMode,
    filePreviewWidth,
    fileTreeLayoutUserSized,
    fileTreePreviewExpanded,
    fileTreeSplitActive,
    layoutProfile,
    layoutWidthLive,
    previewColumnActive,
    rightDockWidth,
    workspacePanelOpen,
  ]);

  const closeWorkspacePanelImmediate = useCallback(() => {
    if (dockCloseTimerRef.current) {
      clearTimeout(dockCloseTimerRef.current);
      dockCloseTimerRef.current = null;
    }
    setDockClosing(false);
    setDockAnimWidth(0);
    setWorkspacePanelOpen(false);
  }, []);

  const openPreviewColumn = useCallback(
    (mode: PreviewMode, options?: { keepWorkspace?: boolean }) => {
      previewWidthUserSizedRef.current = false;
      setPreviewMode(mode);
      setPreviewColumnOpen(true);
      setWorkspacePanelOpen(true);
      if (mode === "terminal") {
        setPreviewTerminalDocked(true);
      }
      void options;
    },
    [],
  );

  const clearInlineFilePreview = useCallback(() => {
    setFilePreviewPath(null);
    setFilePreviewDiff(null);
    setPagePreviewPath(null);
    setFileTreePreviewExpanded(false);
    setFilePreviewComposerOpen(false);
    setPreviewColumnExpanded(false);
  }, []);

  const closePreviewColumn = useCallback(() => {
    setPreviewColumnOpen(false);
    setPreviewColumnExpanded(false);
    setFileTreePreviewExpanded(false);
    setFilePreviewPath(null);
    setFilePreviewDiff(null);
    setFilePreviewComposerOpen(false);
    if (previewTerminalDocked) {
      minimizeTerminalPanel();
      setPreviewTerminalDocked(false);
    }
  }, [minimizeTerminalPanel, previewTerminalDocked]);

  const togglePreviewColumnExpanded = useCallback(() => {
    setPreviewColumnExpanded((value) => {
      if (value) setFilePreviewComposerOpen(false);
      return !value;
    });
  }, []);

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
    if (!workspacePanelOpen && !previewColumnOpen) {
      return;
    }
    closePreviewColumn();
    setDockClosing(true);
    setDockAnimWidth(0);
    setPreviewColumnExpanded(false);
    if (dockCloseTimerRef.current) clearTimeout(dockCloseTimerRef.current);
    dockCloseTimerRef.current = setTimeout(() => {
      setWorkspacePanelOpen(false);
      setDockClosing(false);
      dockCloseTimerRef.current = null;
    }, MOTION_DURATION_NORMAL_MS);
  }, [closePreviewColumn, previewColumnOpen, workspacePanelOpen]);

  const toggleBrowserPreviewExpanded = togglePreviewColumnExpanded;

  const closeFilePreview = useCallback(() => {
    setFilePreviewPath(null);
    setFilePreviewDiff(null);
    setPreviewColumnExpanded(false);
    setFileTreePreviewExpanded(false);
    setFilePreviewComposerOpen(false);
    if (!pagePreviewActive && !browserPreviewActive && !(previewTerminalDocked && terminalHasSessions)) {
      setPreviewColumnOpen(false);
    } else if (previewMode === "file") {
      setPreviewMode(pagePreviewActive ? "page" : browserPreviewActive ? "browser" : "terminal");
    }
  }, [browserPreviewActive, pagePreviewActive, previewMode, previewTerminalDocked, terminalHasSessions]);

  const exitExpandedPreviewComposer = useCallback(() => {
    setFilePreviewComposerOpen(false);
    setPreviewColumnExpanded(false);
  }, []);

  const toggleFilePreviewExpanded = togglePreviewColumnExpanded;

  const expandFileTreePreview = useCallback(() => {
    const target = clampRightDockWidth(studioSinglePanelTargetWidth(layoutWidthLive, chatMode, layoutProfile));
    setFileTreePreviewExpanded(true);
    setRightDockWidth(target);
    setDockAnimWidth(target);
    saveLayoutSize("rightDockWidth", target, clampRightDockWidth);
  }, [chatMode, layoutWidthLive]);

  const collapseFileTreePreview = useCallback(() => {
    setFileTreePreviewExpanded(false);
    setFileTreeLayoutUserSized(false);
    dockWidthUserSizedRef.current = false;
    const target = clampRightDockWidth(studioSinglePanelTargetWidth(layoutWidthLive, chatMode, layoutProfile) * 0.55);
    setRightDockWidth(target);
    setDockAnimWidth(target);
    saveLayoutSize("rightDockWidth", target, clampRightDockWidth);
  }, [chatMode, layoutWidthLive]);

  const backToFileTreeFromPreview = collapseFileTreePreview;

  const togglePreviewTerminal = useCallback(() => {
    if (previewColumnOpen && previewMode === "terminal") {
      closePreviewColumn();
      return;
    }
    openPreviewColumn("terminal");
    if (terminalHasSessions) {
      restoreTerminalPanel();
      return;
    }
    void openNewTerminal();
  }, [
    closePreviewColumn,
    openNewTerminal,
    openPreviewColumn,
    previewColumnOpen,
    previewMode,
    restoreTerminalPanel,
    terminalHasSessions,
  ]);

  const openWebPreview = useCallback(
    (url?: string, options?: { forceNewTab?: boolean }) => {
      const href = url?.trim();
      if (href) {
        const id = openBrowserTab(href);
        setActiveBrowserTabId(id);
      } else if (shouldOpenNewBrowserTab(browserActive, options)) {
        const id = openBrowserTab();
        setActiveBrowserTabId(id);
      }
      openPreviewColumn("browser");
      setSidebarBodyTab("browser");
    },
    [browserActive, openBrowserTab, openPreviewColumn, setActiveBrowserTabId],
  );

  const openPagePreview = useCallback(
    (path?: string) => {
      if (path?.trim()) setPagePreviewPath(path.trim());
      openPreviewColumn("page");
      setSidebarBodyTab("files");
    },
    [openPreviewColumn],
  );

  const openDockTab = useCallback(
    (tab: RightDockTab, options?: { toggle?: boolean }) => {
      if (tab === "browser") {
        openWebPreview();
        return;
      }
      if (tab === "page") {
        openPagePreview();
        return;
      }
      clearInlineFilePreview();
      saveDockTabSelection(tab);
      const panelTab = rightDockTabToSidebar(tab);
      setLastPanelTab(panelTab);
      setSidebarBodyTab(panelTab);
      setRightDockMode(tab);
      const shouldToggle = options?.toggle !== false;
      if (shouldToggle && workspacePanelOpen && rightDockMode === tab && sidebarBodyTab === panelTab) {
        closeWorkspacePanel();
        return;
      }
      if (!browserActive && !terminalHasSessions) {
        setPreviewColumnOpen(false);
      }
      if (!workspacePanelOpen) {
        dockWidthUserSizedRef.current = false;
      }
      setWorkspacePanelOpen(true);
    },
    [
      browserActive,
      clearInlineFilePreview,
      closeWorkspacePanel,
      openPagePreview,
      openWebPreview,
      rightDockMode,
      sidebarBodyTab,
      terminalHasSessions,
      workspacePanelOpen,
    ],
  );

  const openFilePreview = useCallback(
    (path: string, dockTab: RightDockTab = "files") => {
      previewWidthUserSizedRef.current = false;
      dockWidthUserSizedRef.current = false;
      setFileTreeLayoutUserSized(false);
      setFileTreePreviewExpanded(false);
      setPreviewColumnExpanded(false);
      setFilePreviewComposerOpen(false);
      skipDockOpenAnimationRef.current = true;

      if (isPreviewablePagePath(path) && dockTab === "files") {
        setPagePreviewPath(path);
        setPreviewMode("page");
        setPreviewColumnOpen(true);
        saveDockTabSelection("files");
        setRightDockMode("files");
        setSidebarBodyTab("files");
        setWorkspacePanelOpen(true);
        return;
      }

      setFilePreviewPath(path);
      setFilePreviewDiff(null);
      setPreviewMode("file");
      setPreviewColumnOpen(true);
      saveDockTabSelection(dockTab);
      setRightDockMode("files");
      setSidebarBodyTab("files");
      setWorkspacePanelOpen(true);
    },
    [],
  );

  const openActionFilePreview = useCallback(
    (req: ActionFileOpenRequest) => {
      if (filePreviewPath === null) {
        previewWidthUserSizedRef.current = false;
      }
      setPreviewColumnExpanded(false);
      setFilePreviewComposerOpen(false);
      setFilePreviewPath(req.path);
      setFilePreviewDiff(req.diff ?? null);
      openPreviewColumn("file");
      setPreviewColumnOpen(true);
    },
    [filePreviewPath, openPreviewColumn],
  );

  const openPreviewBrowser = useCallback(() => {
    openWebPreview();
  }, [openWebPreview]);

  const togglePreviewBrowser = useCallback(() => {
    if (previewColumnOpen && previewMode === "browser") {
      closePreviewColumn();
      return;
    }
    openWebPreview();
  }, [closePreviewColumn, openWebPreview, previewColumnOpen, previewMode]);

  const togglePreviewPage = useCallback(() => {
    if (previewColumnOpen && previewMode === "page") {
      closePreviewColumn();
      return;
    }
    openPagePreview();
  }, [closePreviewColumn, openPagePreview, previewColumnOpen, previewMode]);

  const togglePreviewMode = useCallback(
    (mode: PreviewMode) => {
      if (mode === "file") {
        if (previewColumnOpen && previewMode === "file") {
          closePreviewColumn();
          return;
        }
        openPreviewColumn("file");
        return;
      }
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
    [
      closePreviewColumn,
      openPreviewColumn,
      previewColumnOpen,
      previewMode,
      togglePreviewBrowser,
      togglePreviewPage,
      togglePreviewTerminal,
    ],
  );

  const openDockHub = useCallback(
    (hub: DockHub) => {
      if (hub === "preview") {
        if (previewColumnOpen) {
          closePreviewColumn();
          return;
        }
        closeWorkspacePanelImmediate();
        previewWidthUserSizedRef.current = false;
        openPreviewColumn(previewMode);
        if (previewMode === "terminal") {
          if (terminalHasSessions) restoreTerminalPanel();
          else void openNewTerminal();
        } else if (previewMode === "browser" && !browserActive) {
          openWebPreview();
        }
        return;
      }

      if (hub === "work") {
        closePreviewColumn();
        dockWidthUserSizedRef.current = false;
      }

      if (workspacePanelOpen && dockHubForTab(rightDockMode) === hub) {
        closeWorkspacePanel();
        return;
      }
      const tab = resolveHubTab(hub);
      openDockTab(tab, { toggle: false });
    },
    [
      browserActive,
      closePreviewColumn,
      closeWorkspacePanel,
      closeWorkspacePanelImmediate,
      openDockTab,
      openNewTerminal,
      openPreviewColumn,
      openWebPreview,
      previewColumnOpen,
      previewMode,
      restoreTerminalPanel,
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

  const measureLayoutWidth = useCallback(() => {
    const live = layoutRef.current?.getBoundingClientRect().width;
    if (live && Number.isFinite(live)) return Math.round(live);
    return layoutWidthLive;
  }, [layoutWidthLive]);

  const startDockResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (!workspacePanelOpen) return;
      setPreviewColumnExpanded(false);
      const startX = event.clientX;
      const startWidth = rightDockWidth;
      resizePartnerRef.current.preview = previewColumnActive ? filePreviewRenderWidth : 0;
      resizePartnerRef.current.dock = startWidth;
      let nextWidth = startWidth;
      const rowChromeLive: WorkbenchRowChrome = {
        previewOpen: previewColumnActive,
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
          const liveLayout = measureLayoutWidth();
          const delta = startX - moveEvent.clientX;
          const maxAllowed = maxStudioDockWidth(liveLayout, resizePartnerRef.current.preview, rowChromeLive, layoutProfile);
          nextWidth = clampDockRenderWidth(startWidth + delta, maxAllowed);
          setRightDockWidth(nextWidth);
        },
        onCommit: () => {
          if (fileTreeSplitActive) setFileTreeLayoutUserSized(true);
          dockWidthUserSizedRef.current = true;
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
      filePreviewRenderWidth,
      measureLayoutWidth,
      previewColumnActive,
      rightDockWidth,
      setSavedDockWidth,
      setSavedFilePreviewWidth,
      workspacePanelOpen,
    ],
  );

  const startFilePreviewResize = useCallback(
    (event: ReactPointerEvent<HTMLButtonElement>) => {
      if (!previewColumnActive) return;
      const startX = event.clientX;
      const startWidth = filePreviewRenderWidth;
      resizePartnerRef.current.preview = startWidth;
      resizePartnerRef.current.dock = workspacePanelOpen ? rightDockWidth : 0;
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
          setPreviewColumnExpanded(false);
          setFilePreviewResizing(true);
          setFilePreviewWidth(startWidth);
        },
        onMove: (moveEvent) => {
          const liveLayout = measureLayoutWidth();
          const delta = startX - moveEvent.clientX;
          nextWidth = fitFilePreviewWidth(
            startWidth + delta,
            liveLayout,
            resizePartnerRef.current.dock,
            rowChromeLive,
            layoutProfile,
          );
          setFilePreviewWidth(nextWidth);
        },
        onCommit: () => {
          if (fileTreeSplitActive) setFileTreeLayoutUserSized(true);
          previewWidthUserSizedRef.current = true;
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
      previewColumnActive,
      filePreviewRenderWidth,
      measureLayoutWidth,
      rightDockWidth,
      endPanelResize,
      setSavedDockWidth,
      setSavedFilePreviewWidth,
      workspacePanelOpen,
    ],
  );

  const resizeFilePreviewWithKeyboard = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>) => {
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        setPreviewColumnExpanded(false);
        const delta = event.key === "ArrowRight" ? 16 : -16;
        setSavedFilePreviewWidth(
          fitFilePreviewWidth(filePreviewWidth + delta, layoutWidthLive, workspacePanelOpen ? rightDockWidth : 0, rowChrome, layoutProfile),
        );
      } else if (event.key === "Home") {
        event.preventDefault();
        setPreviewColumnExpanded(false);
        setSavedFilePreviewWidth(FILE_PREVIEW_MIN_WIDTH);
      } else if (event.key === "End") {
        event.preventDefault();
        setPreviewColumnExpanded(false);
        setSavedFilePreviewWidth(
          fitFilePreviewWidth(FILE_PREVIEW_MAX_WIDTH, layoutWidthLive, workspacePanelOpen ? rightDockWidth : 0, rowChrome, layoutProfile),
        );
      }
    },
    [filePreviewWidth, layoutProfile, layoutWidthLive, rightDockWidth, rowChrome, setSavedFilePreviewWidth, workspacePanelOpen],
  );

  const resizeDockWithKeyboard = useCallback(
    (event: KeyboardEvent<HTMLButtonElement>) => {
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
        event.preventDefault();
        setPreviewColumnExpanded(false);
        const delta = event.key === "ArrowRight" ? 16 : -16;
        const next = fitDockWidth(
          rightDockWidth + delta,
          layoutWidthLive,
          previewColumnActive ? filePreviewRenderWidth : 0,
          rowChrome,
          layoutProfile,
        );
        setSavedDockWidth(next);
      } else if (event.key === "Home") {
        event.preventDefault();
        setPreviewColumnExpanded(false);
        const next = fitDockWidth(RIGHT_DOCK_MIN_WIDTH, layoutWidthLive, previewColumnActive ? filePreviewRenderWidth : 0, rowChrome, layoutProfile);
        setSavedDockWidth(next);
      } else if (event.key === "End") {
        event.preventDefault();
        setPreviewColumnExpanded(false);
        const next = fitDockWidth(RIGHT_DOCK_MAX_WIDTH, layoutWidthLive, previewColumnActive ? filePreviewRenderWidth : 0, rowChrome, layoutProfile);
        setSavedDockWidth(next);
      }
    },
    [
      filePreviewRenderWidth,
      layoutProfile,
      layoutWidthLive,
      previewColumnActive,
      rightDockWidth,
      rowChrome,
      setSavedDockWidth,
    ],
  );

  const addPreviewTerminal = useCallback(() => {
    openPreviewColumn("terminal");
    if (terminalHasSessions) restoreTerminalPanel();
    else void openNewTerminal();
    setSidebarBodyTab("terminal");
  }, [openNewTerminal, openPreviewColumn, restoreTerminalPanel, terminalHasSessions]);

  const selectSidebarSession = useCallback(
    (kind: "browser" | "terminal", id: string) => {
      openPreviewColumn(kind);
      if (kind === "browser") {
        setActiveBrowserTabId(id);
      } else {
        setActiveTerminalId(id);
        restoreTerminalPanel();
      }
      setSidebarBodyTab(kind);
      setWorkspacePanelOpen(true);
    },
    [openPreviewColumn, restoreTerminalPanel, setActiveBrowserTabId, setActiveTerminalId],
  );

  const openSidebarNewTerminal = useCallback(() => {
    openPreviewColumn("terminal");
    void openNewTerminal();
    restoreTerminalPanel();
    setSidebarBodyTab("terminal");
  }, [openNewTerminal, openPreviewColumn, restoreTerminalPanel]);

  const addPreviewBrowser = useCallback(() => {
    openWebPreview(undefined, { forceNewTab: true });
  }, [openWebPreview]);

  const resetFilePreviewWidthFromDock = useCallback(() => {
    if (fileTreeSplitActive) {
      setFileTreeLayoutUserSized(false);
      const split = studioFileTreeSplitWidths(layoutWidthLive, chatMode, layoutProfile);
      setFilePreviewWidth(split.preview);
      saveFilePreviewWidth(split.preview);
      setRightDockWidth(split.dock);
      setDockAnimWidth(split.dock);
      saveLayoutSize("rightDockWidth", split.dock, clampRightDockWidth);
      return;
    }
    applyStudioRightPanelDefault("preview");
  }, [applyStudioRightPanelDefault, chatMode, fileTreeSplitActive, layoutWidthLive]);

  const resetDockWidthFromDefault = useCallback(() => {
    if (fileTreeSplitActive) {
      setFileTreeLayoutUserSized(false);
      const split = studioFileTreeSplitWidths(layoutWidthLive, chatMode, layoutProfile);
      setFilePreviewWidth(split.preview);
      saveFilePreviewWidth(split.preview);
      setRightDockWidth(split.dock);
      setDockAnimWidth(split.dock);
      saveLayoutSize("rightDockWidth", split.dock, clampRightDockWidth);
      return;
    }
    applyStudioRightPanelDefault("dock");
  }, [applyStudioRightPanelDefault, chatMode, fileTreeSplitActive, layoutWidthLive]);

  const clearFilePreviewComposerOpen = useCallback(() => {
    setFilePreviewComposerOpen(false);
  }, []);

  const selectSidebarTab = useCallback(
    (tab: SidebarPrimaryTab) => {
      switch (tab) {
        case "changes":
          openDockTab("changes", { toggle: false });
          return;
        case "git":
          openDockTab("git", { toggle: false });
          return;
        case "files":
          openDockTab("files", { toggle: false });
          return;
        case "context":
          openDockTab("context", { toggle: false });
          return;
        case "todo":
          openDockTab("todo", { toggle: false });
          return;
      }
    },
    [openDockTab],
  );

  const openSidebarPanel = useCallback(() => {
    openDockTab(appMode === "write" ? "context" : "changes", { toggle: false });
  }, [appMode, openDockTab]);

  useEffect(() => {
    if (appMode !== "write") return;
    if (isSidebarPanelTabAllowed(sidebarBodyTab, "write")) return;
    if (sidebarBodyTab === "browser") return;
    openDockTab("context", { toggle: false });
  }, [appMode, openDockTab, sidebarBodyTab]);

  const toggleSidebarExpanded = useCallback(() => {
    if (fileTreePreviewExpanded) collapseFileTreePreview();
    else expandFileTreePreview();
  }, [collapseFileTreePreview, expandFileTreePreview, fileTreePreviewExpanded]);

  const layoutStyle = useMemo(
    () =>
      ({
        "--sidebar-render-width": `${sidebarRenderWidth}px`,
        "--studio-drawer-w": `${studioDrawerRenderWidth}px`,
        "--studio-sidebar-w-open": `${sidebarRenderWidth}px`,
        "--file-preview-render-width": `${previewColumnRenderWidth}px`,
        "--preview-column-width": `${previewColumnRenderWidth}px`,
        "--dock-render-width": `${dockGridWidth}px`,
      }) as CSSProperties,
    [dockGridWidth, previewColumnRenderWidth, sidebarRenderWidth, studioDrawerRenderWidth],
  );

  const addWorkspaceTextToComposer = useCallback(
    (text: string, replace = false) => {
      setComposerInsertRequest({ id: Date.now(), text, replace: replace || undefined });
      if (previewColumnExpanded) {
        setFilePreviewComposerOpen(true);
        return;
      }
      setFilePreviewComposerOpen(false);
    },
    [previewColumnExpanded, setComposerInsertRequest],
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
    previewColumnOpen,
    previewColumnActive,
    fileTreeSplitActive,
    fileTreePreviewContext,
    fileTreeLayoutUserSized,
    fileTreePreviewExpanded,
    expandFileTreePreview,
    collapseFileTreePreview,
    backToFileTreeFromPreview,
    previewColumnExpanded,
    previewMode,
    setPreviewMode,
    filePreviewComposerOpen,
    filePreviewResizing,
    rightDockMode,
    rightDockWidth,
    browserPreviewActive,
    browserPreviewVisible,
    pagePreviewActive,
    pagePreviewVisible,
    previewPanelVisible,
    previewPanelActive,
    previewTerminalDocked,
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
    closePreviewColumn,
    openDockTab,
    openFilePreview,
    openActionFilePreview,
    closeFilePreview,
    exitExpandedPreviewComposer,
    toggleFilePreviewExpanded,
    togglePreviewColumnExpanded,
    toggleBrowserPreviewExpanded,
    togglePreviewTerminal,
    togglePreviewMode,
    addPreviewTerminal,
    addPreviewBrowser,
    openDockHub,
    selectSidebarTab,
    selectSidebarSession,
    sidebarBodyTab,
    lastPanelTab,
    toggleSidebarExpanded,
    openSidebarPanel,
    openSidebarNewTerminal,
    sidebarOpen,
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
