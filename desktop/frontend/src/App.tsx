import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent, PointerEvent as ReactPointerEvent } from "react";
import { ShellExpandProvider, useShellExpand } from "./lib/shellExpand";
import { asArray } from "./lib/array";
import { clearLegacyLangPref, normalizeLangPref, readLegacyLangPref, t, useI18n, useT } from "./lib/i18n";
import { useController } from "./lib/useController";
import { app, onProjectTreeChanged, onScheduleTask } from "./lib/bridge";
import { MessageTimeline } from "./components/MessageTimeline";
import type { ActionFileOpenRequest } from "./components/ActionStream";
import { FloatingComposer } from "./components/FloatingComposer";
import { BottomTerminalPanel, clampTerminalPanelHeight, TERMINAL_PANEL_DEFAULT_HEIGHT, type TerminalTab } from "./components/TerminalPanel";
import { closeAllTerminals, closeTerminal, startTerminal } from "./lib/terminalBridge";
import type { CodeReviewState } from "./components/CodeReviewSection";
import type { ReviewMode, ReviewScope } from "./lib/codeReview";
import { Sidebar } from "./components/Sidebar";
import { Topbar, type RightDockTab } from "./components/Topbar";
import { StudioToolRail } from "./components/StudioToolRail";
import { RightDock } from "./components/RightDock";
import { FilePreviewPanel } from "./components/FilePreviewPanel";
import { ApprovalModal } from "./components/ApprovalModal";
import { AskCard } from "./components/AskCard";
import { MemoryPanel } from "./components/MemoryPanel";
import { HistoryPanel } from "./components/HistoryPanel";
import { CapabilitiesPanel } from "./components/CapabilitiesPanel";
import { UpdateBanner } from "./components/UpdateBanner";
import { OnboardingOverlay } from "./components/OnboardingOverlay";
import { SandboxSetupOverlay } from "./components/SandboxSetupOverlay";
import { SideConversation, type SideMessage } from "./components/SideConversation";
import { RequirementDraft } from "./components/RequirementDraft";
import { ModeWorkspaceCenter } from "./components/ModeWorkspaceCenter";
import { buildWriteConversation } from "./lib/writeConversation";
import type { AppMode } from "./lib/appMode";
import { getDefaultAppMode } from "./lib/startupPrefs";
import { parseTodos, type ToolFileDiff } from "./lib/tools";
import { shouldShowTodoPanel } from "./lib/todoVisibility";
import type { ComposerInsertRequest, ComposerWriteContext, MemoryView, Mode, SessionMeta, TabMeta } from "./lib/types";
import { recordRecentWorkspace } from "./lib/workspaceRecents";
import {
  clearStoredCodeWorkspaceRoot,
  getStoredCodeWorkspaceRoot,
  getStoredComposerNoWorkspace,
  isUsableCodeWorkspaceRoot,
  setStoredCodeWorkspaceRoot,
  setStoredComposerNoWorkspace,
} from "./lib/composerWorkspace";
import {
  getInitialWriteWorkspaceRoot,
  getStoredWriteWorkspaceRoot,
  isNoWriteWorkspace,
  isUsableWriteWorkspaceRoot,
  NO_WORKSPACE_VALUE,
  setStoredWriteWorkspaceRoot,
} from "./lib/writeWorkspace";
import { applyWriteModeSkill } from "./lib/writeSkill";
import {
  dockHubForTab,
  isPreviewHubActive,
  loadHubLastTab,
  loadPreviewPanelState,
  resolveHubTab,
  saveDockTabSelection,
  savePreviewPanelState,
  type DockHub,
  type PreviewMode,
} from "./lib/dockHubs";
import { loadLayoutSize, loadOptionalLayoutSize, saveLayoutSize } from "./lib/layoutPreferences";
import {
  applyTheme,
  clearLegacyThemePreference,
  getTheme,
  getThemeStyle,
  normalizeThemePreference,
  normalizeThemeStyleForTheme,
  readLegacyThemePreference,
  type Theme,
} from "./lib/theme";
import { syncDesktopGitSettings } from "./lib/desktopGitPrefs";
import {
  markLocalAppearanceMigrated,
  readLocalAppearanceForMigration,
  syncAppearanceFromSettings,
} from "./lib/appearancePrefs";
import {
  markLocalCodeReviewMigrated,
  readLocalCodeReviewForMigration,
  syncCodeReviewSettings,
} from "./lib/codeReviewPrefs";
import { GITHUB_CLI_SETTINGS_EVENT } from "./lib/gitHubCliSettingsNav";
import { useWindowStatePersistence } from "./lib/windowState";

const SIDEBAR_COLLAPSED_KEY = "arcdesk.sidebar.collapsed";
const PROJECT_DRAWER_OPEN_KEY = "arcdesk.studio.projectDrawerOpen";
const STUDIO_RAIL_WIDTH = 76;
const STUDIO_DRAWER_WIDTH = 280;
const CHAT_MIN_WIDTH = 760;
/** Minimum chat width reserved when sizing the right dock — lower than CHAT_MIN_WIDTH so the dock can open with the project drawer visible. */
const DOCK_CHAT_MIN_WIDTH = 420;
const WORKSPACE_RESIZER_WIDTH = 8;

function isThemeMode(value: string): value is Theme {
  return value === "auto" || value === "light" || value === "dark";
}
const RIGHT_DOCK_DEFAULT_WIDTH = 300;
const RIGHT_DOCK_DEFAULT_RATIO = 0.2;
const RIGHT_DOCK_MIN_WIDTH = 280;
const RIGHT_DOCK_MAX_WIDTH = 720;
const RIGHT_DOCK_MIN_RENDER_WIDTH = 200;
const FILE_PREVIEW_MIN_WIDTH = RIGHT_DOCK_MIN_WIDTH;
const FILE_PREVIEW_MAX_WIDTH = RIGHT_DOCK_MAX_WIDTH;
const FILE_PREVIEW_DEFAULT_WIDTH = RIGHT_DOCK_DEFAULT_WIDTH;

type HistoryScopeFilter = { scope: "global" | "project"; workspaceRoot: string };
type HistoryViewState =
  | { kind: "history"; source: "scope"; filter: HistoryScopeFilter; sessions: SessionMeta[] }
  | { kind: "history"; source: "all"; sessions: SessionMeta[] }
  | { kind: "trash"; sessions: SessionMeta[] };

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

function loadProjectDrawerOpen(): boolean {
  if (typeof window === "undefined") return true;
  try {
    const stored = window.localStorage.getItem(PROJECT_DRAWER_OPEN_KEY);
    if (stored === "1") return true;
    if (stored === "0") return false;
    // Legacy key inverted collapsed ↔ drawer open.
    const legacyCollapsed = window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY);
    if (legacyCollapsed === "1") return false;
    if (legacyCollapsed === "0") return true;
    return true;
  } catch {
    return true;
  }
}

function saveProjectDrawerOpen(open: boolean): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(PROJECT_DRAWER_OPEN_KEY, open ? "1" : "0");
    window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, open ? "0" : "1");
  } catch {
    /* ignore storage failures */
  }
}

function tabWorkspaceTitle(tab?: TabMeta): string {
  if (!tab) return "Global";
  if (tab.scope === "project") return tab.workspaceName || tab.workspaceRoot || "Project";
  if (tab.scope === "global") return tab.workspaceName || "Global";
  return tab.workspaceName || tab.workspaceRoot || "Global";
}

function topicTitle(tab?: TabMeta): string {
  if (!tab) return "Global";
  const workspaceTitle = tabWorkspaceTitle(tab);
  const topic = tab.topicTitle || (tab.scope === "global" ? workspaceTitle : "Untitled");
  return topic === workspaceTitle ? workspaceTitle : `${workspaceTitle} / ${topic}`;
}

function topicScopeLabel(tab?: TabMeta): string {
  if (!tab) return t("scope.global");
  if (tab.scope === "global") return tab.workspaceName || t("scope.global");
  return t("scope.project", { name: tab.workspaceName || tab.workspaceRoot || "Project" });
}

function normalizeModeValue(mode?: string): Mode {
  return mode === "plan" || mode === "yolo" ? mode : "normal";
}

function sessionsForScope(sessions: SessionMeta[], filter: HistoryScopeFilter): SessionMeta[] {
  if (filter.scope === "project") {
    return sessions.filter((session) => session.scope === "project" && session.workspaceRoot === filter.workspaceRoot);
  }
  return sessions.filter((session) => (session.scope || "global") === "global");
}


/** Global hotkey handler for shell-expand toggle (Ctrl/Cmd+B). */
function ShellHotkeys() {
  const shellExpand = useShellExpand();
  useEffect(() => {
    if (!shellExpand) return;
    const onKey = (e: globalThis.KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "b") {
        e.preventDefault();
        shellExpand.toggleLast();
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [shellExpand]);
  return null;
}

export default function App() {
  const {
    state,
    activeTabId,
    send,
    runShell,
    notice,
    cancel,
    approve,
    answerQuestion,
    setControllerMode,
    newSession,
    listSessions,
    listTrashedSessions,
    resumeSession,
    previewSession,
    deleteSession,
    restoreSession,
    purgeTrashedSession,
    renameSession,
    refreshMeta,
    pickWorkspace,
    switchWorkspace,
    setModel,
    setEffort,
    fetchMemory,
    remember,
    forget,
    saveDoc,
    openProjectTab,
    openGlobalTab,
    syncActiveTab,
    rewind,
  } = useController();
  const { locale, setPref: setLocalePref } = useI18n();
  const t = useT();
  const [modesByTab, setModesByTab] = useState<Record<string, Mode>>({});
  const [tabMetas, setTabMetas] = useState<TabMeta[]>([]);
  // null until the mount probe resolves; true shows the overlay. Probed once —
  // clearing the key mid-session is the Settings panel's job, not the gate's.
  const [needsOnboarding, setNeedsOnboarding] = useState<boolean | null>(null);
  const [sandboxSetup, setSandboxSetup] = useState<null | { reason: "yolo" | "manual" }>(null);
  const pendingYoloRef = useRef(false);
  const [memView, setMemView] = useState<MemoryView | null>(null);
  const [histView, setHistView] = useState<HistoryViewState | null>(null);
  const [projectDrawerOpen, setProjectDrawerOpen] = useState(loadProjectDrawerOpen);
  const [workspacePanelOpen, setWorkspacePanelOpen] = useState(false);
  const [rightDockWidth, setRightDockWidth] = useState(loadRightDockWidth);
  const [dockResizing, setDockResizing] = useState(false);
  const [filePreviewPath, setFilePreviewPath] = useState<string | null>(null);
  const [filePreviewDiff, setFilePreviewDiff] = useState<ToolFileDiff | null>(null);
  const [filePreviewWidth, setFilePreviewWidth] = useState(loadFilePreviewWidth);
  const [filePreviewExpanded, setFilePreviewExpanded] = useState(false);
  const [filePreviewComposerOpen, setFilePreviewComposerOpen] = useState(false);
  const [filePreviewResizing, setFilePreviewResizing] = useState(false);
  const [browserPreviewExpanded, setBrowserPreviewExpanded] = useState(false);
  const [rightDockMode, setRightDockMode] = useState<RightDockTab>(() => loadHubLastTab("context"));
  const [projectRevision, setProjectRevision] = useState(0);
  const [composerInsertRequest, setComposerInsertRequest] = useState<ComposerInsertRequest | null>(null);
  const [capsOpen, setCapsOpen] = useState(false);
  const [appMode, setAppMode] = useState<AppMode>(() => getDefaultAppMode());
  const [writeWorkspaceRoot, setWriteWorkspaceRoot] = useState(() => getInitialWriteWorkspaceRoot());
  const [composerNoWorkspace, setComposerNoWorkspace] = useState(() => getStoredComposerNoWorkspace());
  const [sddOpen, setSddOpen] = useState(false);
  const [goalLabel, setGoalLabel] = useState<string>("");
  const [sideConversationCount, setSideConversationCount] = useState(0);
  const [sideMessages, setSideMessages] = useState<SideMessage[]>([]);
  const [codeReview, setCodeReview] = useState<CodeReviewState>({ status: "idle", mode: "standard", scope: "all" });
  const [renamingTopicId, setRenamingTopicId] = useState<string | null>(null);
  const [topicTitleDraft, setTopicTitleDraft] = useState("");
  const topicRenameSkipCommitRef = useRef(false);
  const topicRenameCommitHandledRef = useRef(false);
  const codeWorkspaceRestoredRef = useRef(false);
  const prevAppModeRef = useRef<AppMode>(appMode);

  // Persist window geometry across launches.
  useWindowStatePersistence();

  useEffect(() => {
    void app.Platform().then((platform) => {
      if (platform === "darwin") {
        document.documentElement.setAttribute("data-platform", "darwin");
      }
    });
  }, []);

  useEffect(() => {
    let cancelled = false;
    const syncDesktopPreferences = async () => {
      const legacyLanguage = readLegacyLangPref();
      const legacyTheme = readLegacyThemePreference();
      if (legacyLanguage || legacyTheme.hasValue) {
        await app.MigrateDesktopPreferences(legacyLanguage, legacyTheme.theme, legacyTheme.style);
        clearLegacyLangPref();
        clearLegacyThemePreference();
      }
      const localAppearance = readLocalAppearanceForMigration();
      const localCodeReview = readLocalCodeReviewForMigration();
      if (localAppearance.hasValue || localCodeReview.hasValue) {
        await app.MigrateDesktopLocalPrefs({
          backgroundPreset: localAppearance.view.backgroundPreset,
          foregroundPreset: localAppearance.view.foregroundPreset,
          textSize: localAppearance.view.textSize,
          codeFontSize: localAppearance.view.codeFontSize,
          diffMarker: localAppearance.view.diffMarker,
          codeReviewScope: localCodeReview.scope,
          codeReviewSecurity: localCodeReview.security,
          hasAppearance: localAppearance.hasValue,
          hasCodeReview: localCodeReview.hasValue,
        });
        if (localAppearance.hasValue) markLocalAppearanceMigrated();
        if (localCodeReview.hasValue) markLocalCodeReviewMigrated();
      }
      const settings = await app.Settings();
      if (cancelled) return;
      const nextTheme = normalizeThemePreference(settings.desktopTheme);
      const nextStyle = normalizeThemeStyleForTheme(settings.desktopThemeStyle, nextTheme);
      applyTheme(nextTheme, nextStyle, { syncSurfaces: false });
      syncAppearanceFromSettings(settings.desktopAppearance);
      syncDesktopGitSettings(settings.desktopGit);
      syncCodeReviewSettings(settings.desktopCodeReview);
      setLocalePref(normalizeLangPref(settings.desktopLanguage));
    };
    void syncDesktopPreferences().catch((e) => {
      console.warn("desktop preferences sync failed", e);
    });
    return () => {
      cancelled = true;
    };
  }, [setLocalePref]);

  // Open settings when the native menu item (CmdOrCtrl+,) is activated.
  useEffect(() => {
    if (typeof window === "undefined" || !window.runtime) return;
    return window.runtime.EventsOn("app:open-settings", () => {
      setAppMode("settings");
    });
  }, []);
  useEffect(() => {
    const openGitHubCliSettings = () => setAppMode("settings");
    window.addEventListener(GITHUB_CLI_SETTINGS_EVENT, openGitHubCliSettings);
    return () => window.removeEventListener(GITHUB_CLI_SETTINGS_EVENT, openGitHubCliSettings);
  }, []);
  const [pendingPlanRevision, setPendingPlanRevision] = useState<string | null>(null);
  const [terminalOpen, setTerminalOpen] = useState(false);
  const [terminalTabs, setTerminalTabs] = useState<TerminalTab[]>([]);
  const [activeTerminalId, setActiveTerminalId] = useState<string | null>(null);
  const [terminalHeight, setTerminalHeight] = useState(() =>
    loadLayoutSize("terminalPanelHeight", TERMINAL_PANEL_DEFAULT_HEIGHT, clampTerminalPanelHeight),
  );
  const [footerHeight, setFooterHeight] = useState(0);
  const layoutRef = useRef<HTMLDivElement>(null);
  const footerRef = useRef<HTMLDivElement>(null);
  const terminalTabKeyRef = useRef(0);
  const [layoutWidth, setLayoutWidth] = useState(0);
  const preferredWorkspacePanelWidth = rightDockWidth;
  const filePreviewOpen = filePreviewPath !== null;
  const chatMode = appMode === "code";
  const showWorkbenchFooter = chatMode || appMode === "write";
  const showRightDock = chatMode || appMode === "write";

  useEffect(() => {
    if (chatMode) return;
    setRenamingTopicId(null);
    setTopicTitleDraft("");
  }, [chatMode]);

  useEffect(() => {
    if (appMode !== "write") return;
    void app.EnsureBundledSkills().catch(() => {});
  }, [appMode]);

  useEffect(() => {
    setComposerInsertRequest(null);
  }, [appMode]);

  const sidebarRenderWidth = projectDrawerOpen
    ? STUDIO_RAIL_WIDTH + STUDIO_DRAWER_WIDTH
    : STUDIO_RAIL_WIDTH;
  const measuredMainWidth = layoutWidth > 0
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
  const workspacePanelRenderable = workspacePanelOpen && workspacePanelRenderWidth > 0;
  const dockGridWidth = 0;

  const activeTab = useMemo(
    () => tabMetas.find((tab) => tab.id === activeTabId) ?? tabMetas.find((tab) => tab.active),
    [activeTabId, tabMetas],
  );
  const mode = activeTabId ? modesByTab[activeTabId] ?? "normal" : "normal";
  const setMode = useCallback(
    (next: Mode | ((prev: Mode) => Mode)) => {
      if (!activeTabId) return;
      setModesByTab((current) => {
        const prev = current[activeTabId] ?? "normal";
        const value = typeof next === "function" ? next(prev) : next;
        if (value === prev) return current;
        return { ...current, [activeTabId]: value };
      });
    },
    [activeTabId],
  );
  const topicbarEditing = Boolean(activeTab?.topicId && activeTab.topicId === renamingTopicId);

  useEffect(() => {
    const ids = new Set(tabMetas.map((tab) => tab.id));
    setModesByTab((current) => {
      let changed = false;
      const next: Record<string, Mode> = {};
      for (const tab of tabMetas) {
        const mode = normalizeModeValue(tab.mode);
        next[tab.id] = mode;
        if (current[tab.id] !== mode) changed = true;
      }
      for (const id of Object.keys(current)) {
        if (!ids.has(id)) changed = true;
      }
      return changed ? next : current;
    });
  }, [tabMetas]);

  useEffect(() => {
    if (!renamingTopicId || activeTab?.topicId === renamingTopicId) return;
    topicRenameSkipCommitRef.current = false;
    topicRenameCommitHandledRef.current = false;
    setRenamingTopicId(null);
    setTopicTitleDraft("");
  }, [activeTab?.topicId, renamingTopicId]);

  const syncModeToController = useCallback((m: Mode) => setControllerMode(m), [setControllerMode]);

  useEffect(() => {
    void app.SetTrayLocale(locale).catch(() => {});
  }, [locale]);

  // applyMode is the single source of truth for the input mode: it updates the
  // local pill and pushes the matching gate state to the controller (plan = read
  // only; yolo = auto-approve every tool call). normal clears both.
  const applyMode = useCallback(
    (m: Mode) => {
      if (m === "yolo") {
        void app
          .ProjectSandboxStatus()
          .then((status) => {
            if (!status.configured) {
              pendingYoloRef.current = true;
              setSandboxSetup({ reason: "yolo" });
              return;
            }
            setMode(m);
            void syncModeToController(m);
          })
          .catch(() => {
            pendingYoloRef.current = true;
            setSandboxSetup({ reason: "yolo" });
          });
        return;
      }
      setMode(m);
      void syncModeToController(m);
    },
    [syncModeToController],
  );
  // Shift+Tab cycles auto(normal) → plan → yolo → auto.
  const cycleMode = useCallback(() => {
    applyMode(mode === "normal" ? "plan" : mode === "plan" ? "yolo" : "normal");
  }, [mode, applyMode]);

  // Switching models rebuilds the controller, which starts in normal mode — so
  // re-apply the current mode, or the pill would say plan/YOLO while the fresh
  // controller silently uses normal gating.
  const switchModel = useCallback(
    async (name: string) => {
      await setModel(name);
      await syncModeToController(mode);
    },
    [setModel, mode, syncModeToController],
  );

  // Startup and workspace/model rebuilds create a fresh controller in normal
  // mode. Re-apply the UI mode once the controller is ready, including the case
  // where the user picked YOLO while boot was still loading and SetBypass was a
  // harmless no-op.
  useEffect(() => {
    if (state.meta?.ready !== true || mode === "normal") return;
    void syncModeToController(mode);
  }, [state.meta, mode, syncModeToController]);

  // The live task list pinned above the composer comes from the most recent
  // successful top-level todo_write result; failed or still-running attempts do
  // not advance the canonical panel state. It stays visible through the final
  // all-completed update, and can be dismissed by the user (the ✕). A dismissal
  // is keyed to that list's id, so a fresh accepted todo_write brings the panel
  // back.
  const todoEntry = useMemo(() => {
    for (let i = state.items.length - 1; i >= 0; i--) {
      const it = state.items[i];
      if (it.kind === "tool" && it.name === "todo_write" && !it.parentId && it.status === "done" && !it.error) {
        return { item: it, index: i };
      }
    }
    return null;
  }, [state.items]);
  const todoItem = todoEntry?.item ?? null;
  const todos = useMemo(() => (todoItem ? parseTodos(todoItem.args) : []), [todoItem]);
  const [dismissedTodo, setDismissedTodo] = useState<string | null>(null);
  const showTodos = shouldShowTodoPanel(todoItem?.id, dismissedTodo, todos);
  const [todoNow, setTodoNow] = useState(() => Date.now());
  const todoSeenRef = useRef<{ id: string; at: number } | null>(null);

  useEffect(() => {
    if (!todoItem) {
      todoSeenRef.current = null;
      return;
    }
    if (todoSeenRef.current?.id !== todoItem.id) {
      todoSeenRef.current = { id: todoItem.id, at: Date.now() };
      setTodoNow(Date.now());
    }
  }, [todoItem]);

  useEffect(() => {
    if (!showTodos) return;
    const id = window.setInterval(() => setTodoNow(Date.now()), 15000);
    return () => window.clearInterval(id);
  }, [showTodos]);

  const todoStale = useMemo(() => {
    if (!showTodos || !todoEntry) return false;
    const after = state.items.slice(todoEntry.index + 1);
    const completedToolsAfter = after.filter(
      (it) => it.kind === "tool" && it.name !== "todo_write" && !it.parentId && (it.status === "done" || it.status === "error"),
    ).length;
    const finalAssistantAfter = after.some((it) => it.kind === "assistant" && !it.streaming && it.text.trim() !== "");
    const readinessNoticeAfter = after.some(
      (it) => it.kind === "notice" && /final-answer readiness|todo_write|complete_step/i.test(it.text),
    );
    const staleByTime = state.running && todoSeenRef.current?.id === todoEntry.item.id && todoNow - todoSeenRef.current.at > 90_000;
    return completedToolsAfter >= 2 || finalAssistantAfter || readinessNoticeAfter || staleByTime;
  }, [showTodos, state.items, state.running, todoEntry, todoNow]);

  // useDeferredValue lets React prioritise Composer input (high-priority) over
  // Transcript re-renders (low-priority) during streaming. When a keystroke
  // and a transcript update collide, the keystroke is processed immediately
  // and the transcript re-render is deferred to idle time.
  const deferredItems = useDeferredValue(state.items);

  const writeConversationTurns = useMemo(() => {
    if (appMode !== "write") return [];
    return buildWriteConversation(state.items, state.live);
  }, [appMode, state.items, state.live]);

  useEffect(() => {
    if (!pendingPlanRevision || state.running) return;
    const text = pendingPlanRevision;
    setPendingPlanRevision(null);
    send(text);
  }, [pendingPlanRevision, send, state.running]);

  // Memory drawer: opening fetches a fresh snapshot; writes re-fetch so the
  // panel reflects what landed on disk.
  const openMemory = useCallback(async () => {
    setMemView(await fetchMemory());
  }, [fetchMemory]);

  const closeMemory = useCallback(() => setMemView(null), []);

  const closeWorkspacePanel = useCallback(() => {
    if (!workspacePanelOpen) {
      return;
    }
    setWorkspacePanelOpen(false);
    setBrowserPreviewExpanded(false);
    setFilePreviewPath(null);
  }, [workspacePanelOpen]);

  const browserPreviewOpen = workspacePanelOpen && rightDockMode === "browser";

  useEffect(() => {
    savePreviewPanelState({ terminal: terminalOpen, browser: browserPreviewOpen });
  }, [browserPreviewOpen, terminalOpen]);

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

  const closeBrowserPreview = useCallback(() => {
    setBrowserPreviewExpanded(false);
    if (browserPreviewOpen) {
      closeWorkspacePanel();
    }
  }, [browserPreviewOpen, closeWorkspacePanel]);

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
      if (tab !== "browser") {
        setBrowserPreviewExpanded(false);
      }
      setRightDockMode(tab);
      setWorkspacePanelOpen(true);
    },
    [closeWorkspacePanel, rightDockMode, workspacePanelOpen],
  );

  const openFilePreview = useCallback(
    (path: string, dockTab: RightDockTab = "files") => {
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

  const openPreviewBrowser = useCallback(() => {
    openDockTab("browser", { toggle: false });
    savePreviewPanelState({ terminal: terminalOpen, browser: true });
  }, [openDockTab, terminalOpen]);

  const togglePreviewBrowser = useCallback(() => {
    if (browserPreviewOpen) {
      closeBrowserPreview();
      return;
    }
    openPreviewBrowser();
  }, [browserPreviewOpen, closeBrowserPreview, openPreviewBrowser]);

  const deactivatePreview = useCallback(() => {
    if (terminalOpen) {
      closeTerminalPanel();
    }
    if (browserPreviewOpen) {
      closeBrowserPreview();
    }
  }, [browserPreviewOpen, closeBrowserPreview, closeTerminalPanel, terminalOpen]);

  const togglePreviewMode = useCallback(
    (mode: PreviewMode) => {
      if (mode === "terminal") {
        togglePreviewTerminal();
        return;
      }
      togglePreviewBrowser();
    },
    [togglePreviewBrowser, togglePreviewTerminal],
  );

  const openDockHub = useCallback(
    (hub: DockHub) => {
      if (hub === "preview") {
        if (browserPreviewOpen) {
          deactivatePreview();
          return;
        }
        if (isPreviewHubActive(terminalOpen, workspacePanelOpen, rightDockMode)) {
          deactivatePreview();
          return;
        }
        openPreviewBrowser();
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
      browserPreviewOpen,
      closeWorkspacePanel,
      deactivatePreview,
      openDockTab,
      openNewTerminal,
      openPreviewBrowser,
      rightDockMode,
      terminalOpen,
      workspacePanelOpen,
    ],
  );

  const clearCodeReview = useCallback(() => {
    setCodeReview((current) => ({ status: "idle", mode: current.mode, scope: current.scope }));
  }, []);

  const runCodeReview = useCallback(
    async (reviewMode: ReviewMode, scope: ReviewScope, paths: string[]) => {
      if (state.running) {
        notice(t("changes.reviewBusy"), "warn");
        return;
      }
      if (!paths.length) {
        notice(t("changes.reviewNoFiles"), "warn");
        return;
      }
      setAppMode("code");
      openDockTab("changes", { toggle: false });
      setCodeReview({ status: "running", mode: reviewMode, scope });
      try {
        const result = await app.RunCodeReview(reviewMode, scope, paths);
        if (result.err) {
          setCodeReview((current) => ({
            ...current,
            status: "error",
            error: result.err,
            finishedAt: Date.now(),
          }));
          return;
        }
        const text = result.text.trim();
        if (!text) {
          setCodeReview((current) => ({
            ...current,
            status: "error",
            error: t("changes.reviewEmptyResult"),
            finishedAt: Date.now(),
          }));
          return;
        }
        setCodeReview((current) => ({
          ...current,
          status: "done",
          text,
          error: undefined,
          finishedAt: Date.now(),
        }));
      } catch (err) {
        setCodeReview((current) => ({
          ...current,
          status: "error",
          error: err instanceof Error ? err.message : String(err),
          finishedAt: Date.now(),
        }));
      }
    },
    [state.running, notice, t, openDockTab],
  );

  // handleSend intercepts the slash commands that need a desktop-native action
  // before they reach the backend: "/model <ref>" rebuilds on that model, and
  // "/memory" opens the memory drawer. Everything else — skills (/init, …),
  // custom commands, bare /model and the other read-only management verbs
  // (/skill, /hooks, /mcp) — goes straight to Submit, which the controller
  // resolves (a turn, or a listing Notice).
  const handleSend = useCallback(
    async (displayText: string, submitText = displayText) => {
      const trimmed = displayText.trim();
      // "!<cmd>" runs a shell command directly, bypassing the model.
      if (trimmed.startsWith("!")) {
        const cmd = trimmed.slice(1).trim();
        if (!cmd) {
          notice("usage: !<command>  (e.g. !ls -la)");
          return;
        }
        runShell(cmd);
        return;
      }
      const model = /^\/model\s+(\S+)$/.exec(trimmed);
      if (model) {
        void switchModel(model[1]);
        return;
      }
      if (trimmed === "/memory") {
        void openMemory();
        return;
      }
      const goal = /^\/goal\s+(.+)$/.exec(trimmed);
      if (goal) {
        setGoalLabel(goal[1].trim());
        notice(t("goal.set", { label: goal[1].trim() }));
        return;
      }
      const btw = /^\/btw\s+(.+)$/.exec(trimmed);
      if (btw) {
        const text = btw[1].trim();
        const entry: SideMessage = { id: `side-${Date.now()}`, text, outgoing: true, createdAt: Date.now() };
        setSideMessages((current) => [...current, entry]);
        setSideConversationCount((value) => value + 1);
        notice(t("sideChat.opened"));
        return;
      }
      if (trimmed === "/review" || trimmed === "/review run") {
        setAppMode("code");
        openDockTab("changes", { toggle: false });
        if (trimmed === "/review run") {
          void app.WorkspaceChanges().then((view) => {
            const paths = view.files.map((file) => file.path);
            void runCodeReview("standard", "all", paths);
          });
        } else {
          notice(t("slash.reviewOpened"));
        }
        return;
      }
      if (trimmed === "/sdd") {
        setSddOpen(true);
        notice(t("slash.sddOpened"));
        return;
      }
      const theme = /^\/theme(?:\s+(\S+))?$/.exec(trimmed);
      if (theme) {
        const arg = theme[1]?.toLowerCase();
        if (!arg) {
          notice(t("settings.themeCurrentSimple", { theme: getTheme() }));
          return;
        }
        if (isThemeMode(arg)) {
          const next = arg;
          const nextStyle = normalizeThemeStyleForTheme(getThemeStyle(), next);
          await app.SetDesktopAppearance(next, nextStyle);
          applyTheme(next, nextStyle, { syncSurfaces: true });
          notice(t("settings.themeChangedSimple", { theme: next }));
          return;
        }
        notice(t("settings.themeUnknown", { name: arg }), "warn");
        return;
      }
      await syncModeToController(mode);
      const outbound =
        appMode === "write"
          ? applyWriteModeSkill(trimmed, submitText.trim())
          : { displayText: trimmed, submitText: submitText.trim() };
      send(outbound.displayText, outbound.submitText);
      if (filePreviewComposerOpen) {
        exitExpandedPreviewComposer();
      }
    },
    [switchModel, openMemory, syncModeToController, mode, send, runShell, notice, t, setGoalLabel, setSideConversationCount, setSideMessages, openDockTab, setSddOpen, setAppMode, filePreviewComposerOpen, exitExpandedPreviewComposer, appMode, runCodeReview],
  );

  const refreshTabMetas = useCallback(async (): Promise<TabMeta[]> => {
    const tabs = asArray(await app.ListTabs().catch(() => [] as TabMeta[]));
    setTabMetas(tabs);
    return tabs;
  }, []);

  useEffect(() => {
    void refreshTabMetas();
    const id = window.setInterval(() => void refreshTabMetas(), 2000);
    return () => window.clearInterval(id);
  }, [refreshTabMetas]);

  useEffect(() => {
    return onProjectTreeChanged(() => {
      setProjectRevision((value) => value + 1);
      void refreshTabMetas();
    });
  }, [refreshTabMetas]);

  useEffect(() => {
    return onScheduleTask((event) => {
      if (event.error) {
        notice(t("schedule.failed", { name: event.name, error: event.error }), "warn");
        return;
      }
      setAppMode("code");
      void refreshTabMetas();
      if (event.source === "auto") {
        notice(t("schedule.autoRan", { name: event.name }));
      }
    });
  }, [notice, refreshTabMetas, t]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const needs = await app.NeedsOnboarding();
        if (!cancelled) setNeedsOnboarding(needs);
      } catch {
        // Bridge unavailable (browser dev seam) — skip the gate; a real key
        // failure still surfaces via the topbar startupError banner.
        if (!cancelled) setNeedsOnboarding(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    setFilePreviewPath(null);
  }, [state.meta?.cwd]);

  useEffect(() => {
    const el = footerRef.current;
    if (!el || typeof ResizeObserver === "undefined") {
      setFooterHeight(0);
      return;
    }
    const update = () => setFooterHeight(Math.round(el.getBoundingClientRect().height));
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, [filePreviewComposerOpen, filePreviewExpanded, chatMode, appMode, terminalOpen, state.approval, state.ask, showWorkbenchFooter]);

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

  const startNewSession = useCallback(async () => {
    await newSession();
  }, [newSession]);

  const closeProjectDrawer = useCallback(() => {
    setProjectDrawerOpen(false);
    saveProjectDrawerOpen(false);
  }, []);

  const toggleProjectDrawer = useCallback(() => {
    setProjectDrawerOpen((open) => {
      const next = !open;
      saveProjectDrawerOpen(next);
      return next;
    });
  }, []);

  const toggleTerminal = useCallback(() => {
    togglePreviewTerminal();
  }, [togglePreviewTerminal]);

  const setSavedTerminalHeight = useCallback((height: number) => {
    const next = clampTerminalPanelHeight(height);
    setTerminalHeight(next);
    saveLayoutSize("terminalPanelHeight", next, clampTerminalPanelHeight);
  }, []);

  useEffect(() => {
    const onKey = (event: globalThis.KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key === "\\") {
        event.preventDefault();
        toggleProjectDrawer();
        return;
      }
      if ((event.metaKey || event.ctrlKey) && event.key === "`") {
        event.preventDefault();
        if (event.shiftKey) {
          void openNewTerminal();
        } else {
          toggleTerminal();
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [toggleProjectDrawer, toggleTerminal, openNewTerminal]);

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

  const layoutStyle = useMemo(
    () =>
      ({
        "--sidebar-render-width": `${sidebarRenderWidth}px`,
        "--file-preview-render-width": `${filePreviewRenderWidth}px`,
        "--dock-render-width": `${dockGridWidth}px`,
      }) as CSSProperties,
    [dockGridWidth, filePreviewRenderWidth, sidebarRenderWidth, projectDrawerOpen],
  );

  const addWorkspaceTextToComposer = useCallback((text: string, replace = false) => {
    setComposerInsertRequest({ id: Date.now(), text, replace: replace || undefined });
    if (filePreviewExpanded) {
      setFilePreviewComposerOpen(true);
      return;
    }
    setFilePreviewComposerOpen(false);
  }, [filePreviewExpanded]);

  const addWriteContextToComposer = useCallback((context: ComposerWriteContext) => {
    if (appMode !== "write") return;
    setComposerInsertRequest({
      id: Date.now(),
      writeContext: context,
      replace: true,
    });
    setFilePreviewComposerOpen(false);
  }, [appMode]);

  const pickWriteWorkspace = useCallback(async () => {
    const picked = await pickWorkspace();
    if (picked) {
      setComposerNoWorkspace(false);
      setStoredComposerNoWorkspace(false);
      setStoredWriteWorkspaceRoot(picked);
      recordRecentWorkspace(picked);
      setWriteWorkspaceRoot(picked);
      await app.AddWriteWorkspace(picked).catch(() => undefined);
    }
    return picked || undefined;
  }, [pickWorkspace]);

  const handleWriteWorkspaceChange = useCallback((root: string) => {
    if (isNoWriteWorkspace(root)) {
      setStoredWriteWorkspaceRoot(NO_WORKSPACE_VALUE);
      setWriteWorkspaceRoot(NO_WORKSPACE_VALUE);
      setComposerNoWorkspace(true);
      setStoredComposerNoWorkspace(true);
      return;
    }
    if (!isUsableWriteWorkspaceRoot(root)) return;
    setComposerNoWorkspace(false);
    setStoredComposerNoWorkspace(false);
    setStoredWriteWorkspaceRoot(root);
    recordRecentWorkspace(root);
    setWriteWorkspaceRoot(root);
  }, []);

  const handleUseNoWorkspace = useCallback(() => {
    setComposerNoWorkspace(true);
    setStoredComposerNoWorkspace(true);
    if (appMode === "write") {
      setWriteWorkspaceRoot(NO_WORKSPACE_VALUE);
      setStoredWriteWorkspaceRoot(NO_WORKSPACE_VALUE);
    } else {
      clearStoredCodeWorkspaceRoot();
    }
  }, [appMode]);

  const handleOpenTopic = useCallback(async (scope: string, workspaceRoot: string, topicId: string) => {
    const trimmedTopicId = topicId.trim();
    if (!trimmedTopicId) return;
    setAppMode("code");
    setHistView(null);
    if (scope === "project" && isUsableCodeWorkspaceRoot(workspaceRoot)) {
      setStoredCodeWorkspaceRoot(workspaceRoot);
      setComposerNoWorkspace(false);
      setStoredComposerNoWorkspace(false);
      recordRecentWorkspace(workspaceRoot);
    }
    const meta =
      scope === "global"
        ? await openGlobalTab(trimmedTopicId, true)
        : await openProjectTab(workspaceRoot, trimmedTopicId, true);
    if (!meta?.id) return;
    await refreshTabMetas();
  }, [openGlobalTab, openProjectTab, refreshTabMetas]);

  // History drawer: project menus can open a scoped saved-session list. Idle row
  // clicks resume; running row clicks only preview through PreviewSession.
  const openAllHistory = useCallback(async () => {
    setHistView({ kind: "history", source: "all", sessions: await listSessions() });
  }, [listSessions]);
  const openProjectHistory = useCallback(async (scope: "global" | "project", workspaceRoot: string) => {
    const sessions = await listSessions();
    setHistView({
      kind: "history",
      source: "scope",
      filter: { scope, workspaceRoot },
      sessions: sessionsForScope(sessions, { scope, workspaceRoot }),
    });
  }, [listSessions]);
  const openTrash = useCallback(async () => {
    setHistView({ kind: "trash", sessions: await listTrashedSessions() });
  }, [listTrashedSessions]);
  const closeHistory = useCallback(() => setHistView(null), []);
  const onResumeSession = useCallback(
    async (session: SessionMeta) => {
      if (state.running) return;
      setAppMode("code");
      setHistView(null);
      const scope = session.scope || (session.workspaceRoot ? "project" : "global");
      let targetTab: TabMeta | undefined;
      if (scope === "project" && session.workspaceRoot && session.topicId) {
        targetTab = await openProjectTab(session.workspaceRoot, session.topicId);
      } else if (scope === "global" && session.topicId) {
        targetTab = await openGlobalTab(session.topicId);
      }
      await resumeSession(session.path, targetTab?.id);
      if (targetTab) {
        await refreshTabMetas();
      }
    },
    [openGlobalTab, openProjectTab, refreshTabMetas, state.running, resumeSession],
  );
  // Delete / rename act on disk, then re-fetch so the panel reflects the change.
  const onDeleteSession = useCallback(
    async (path: string) => {
      if (state.running) return;
      await deleteSession(path);
      const sessions = await listSessions();
      setHistView((cur) =>
        cur === null
          ? null
          : cur.kind === "history"
            ? { ...cur, sessions: cur.source === "scope" ? sessionsForScope(sessions, cur.filter) : sessions }
            : cur,
      );
    },
    [state.running, deleteSession, listSessions],
  );
  const onRenameSession = useCallback(
    async (path: string, title: string) => {
      if (state.running) return;
      await renameSession(path, title);
      const sessions = await listSessions();
      setHistView((cur) =>
        cur === null
          ? null
          : cur.kind === "history"
            ? { ...cur, sessions: cur.source === "scope" ? sessionsForScope(sessions, cur.filter) : sessions }
            : cur,
      );
    },
    [state.running, renameSession, listSessions],
  );
  const onRestoreTrashedSession = useCallback(
    async (path: string) => {
      await restoreSession(path);
      const trashed = await listTrashedSessions();
      setHistView((cur) => (cur === null ? null : { kind: "trash", sessions: trashed }));
    },
    [restoreSession, listTrashedSessions],
  );
  const onPurgeTrashedSession = useCallback(
    async (path: string) => {
      await purgeTrashedSession(path);
      const trashed = await listTrashedSessions();
      setHistView((cur) => (cur === null ? null : { kind: "trash", sessions: trashed }));
    },
    [purgeTrashedSession, listTrashedSessions],
  );
  const onPurgeAllTrashedSessions = useCallback(
    async (paths: string[]) => {
      const uniquePaths = Array.from(new Set(paths));
      for (const path of uniquePaths) {
        await purgeTrashedSession(path);
      }
      const trashed = await listTrashedSessions();
      setHistView((cur) => (cur === null ? null : { kind: "trash", sessions: trashed }));
    },
    [purgeTrashedSession, listTrashedSessions],
  );

  // Workspace: open the folder chooser and switch projects. The hook resets the
  // transcript and refreshes meta on a pick. A cancel is a no-op.
  const switchFolder = useCallback(async (path?: string) => {
    const picked = path === undefined ? await pickWorkspace() : await switchWorkspace(path);
    if (picked) {
      setStoredCodeWorkspaceRoot(picked);
      setComposerNoWorkspace(false);
      setStoredComposerNoWorkspace(false);
      recordRecentWorkspace(picked);
      setProjectRevision((value) => value + 1);
      await refreshTabMetas();
    }
    return picked;
  }, [pickWorkspace, switchWorkspace, refreshTabMetas]);

  useEffect(() => {
    if (!state.meta?.ready) return;
    if (!getStoredCodeWorkspaceRoot() && !getStoredComposerNoWorkspace()) {
      const seed = activeTab?.workspaceRoot?.trim();
      if (seed && isUsableCodeWorkspaceRoot(seed)) {
        setStoredCodeWorkspaceRoot(seed);
      }
    }
  }, [state.meta?.ready, activeTab?.workspaceRoot]);

  useEffect(() => {
    if (!state.meta?.ready || codeWorkspaceRestoredRef.current) return;
    if (composerNoWorkspace) return;
    const stored = getStoredCodeWorkspaceRoot();
    if (!isUsableCodeWorkspaceRoot(stored)) return;
    codeWorkspaceRestoredRef.current = true;
    void switchFolder(stored);
  }, [state.meta?.ready, composerNoWorkspace, switchFolder]);

  useEffect(() => {
    const prev = prevAppModeRef.current;
    prevAppModeRef.current = appMode;
    if (appMode === "write" && prev !== "write") {
      const stored = getStoredWriteWorkspaceRoot();
      if (isNoWriteWorkspace(stored)) {
        setWriteWorkspaceRoot(NO_WORKSPACE_VALUE);
      } else if (isUsableWriteWorkspaceRoot(stored)) {
        setWriteWorkspaceRoot(stored);
      }
      return;
    }
    if (appMode === "code" && prev !== "code" && !composerNoWorkspace) {
      const stored = getStoredCodeWorkspaceRoot();
      if (isUsableCodeWorkspaceRoot(stored)) {
        void switchFolder(stored);
      }
    }
  }, [appMode, composerNoWorkspace, switchFolder]);

  const composerPickFolder = useCallback(
    async (path?: string) => {
      if (appMode === "write") {
        if (path) {
          handleWriteWorkspaceChange(path);
          return path;
        }
        const picked = await pickWriteWorkspace();
        if (picked) {
          setComposerNoWorkspace(false);
          setStoredComposerNoWorkspace(false);
        }
        return picked || "";
      }
      return switchFolder(path);
    },
    [appMode, handleWriteWorkspaceChange, pickWriteWorkspace, switchFolder],
  );

  const removeWorkspace = useCallback(async (path: string) => {
    await app.RemoveWorkspace(path);
    setProjectRevision((value) => value + 1);
    await refreshTabMetas();
  }, [refreshTabMetas]);

  const refreshProjectsAndTabs = useCallback(async () => {
    setProjectRevision((value) => value + 1);
    const tabs = await refreshTabMetas();
    if (activeTabId && !tabs.some((tab) => tab.id === activeTabId)) {
      await syncActiveTab(true);
    }
  }, [activeTabId, refreshTabMetas, syncActiveTab]);

  const renameTopic = useCallback(async (topicId: string, title: string) => {
    const nextTitle = title.trim();
    if (!topicId || !nextTitle) return;
    await app.RenameTopic(topicId, nextTitle);
    await refreshProjectsAndTabs();
  }, [refreshProjectsAndTabs]);

  const workspaceRoot = activeTab?.workspaceRoot || activeTab?.cwd || state.meta?.cwd || ".";
  const composerCwd =
    appMode === "write"
      ? isNoWriteWorkspace(writeWorkspaceRoot)
        ? NO_WORKSPACE_VALUE
        : isUsableWriteWorkspaceRoot(writeWorkspaceRoot)
          ? writeWorkspaceRoot
          : undefined
      : composerNoWorkspace
        ? NO_WORKSPACE_VALUE
        : state.meta?.cwd;
  const composerWorkspaceNone =
    appMode === "write" ? isNoWriteWorkspace(writeWorkspaceRoot) : composerNoWorkspace;
  const topbarWorkspacePath =
    chatMode && !composerNoWorkspace
      ? activeTab?.workspaceRoot || getStoredCodeWorkspaceRoot() || undefined
      : undefined;
  const showTopbarWorkspace = isUsableCodeWorkspaceRoot(topbarWorkspacePath);
  const activeTerminalTab = useMemo(() => {
    if (!resolvedActiveTerminalId) return null;
    return terminalTabs.find((tab) => tab.id === resolvedActiveTerminalId) ?? null;
  }, [resolvedActiveTerminalId, terminalTabs]);

  const handleSideSend = useCallback((text: string) => {
    const trimmed = text.trim();
    if (!trimmed) return;
    setSideMessages((current) => [
      ...current,
      { id: `side-${Date.now()}`, text: trimmed, outgoing: true, createdAt: Date.now() },
    ]);
    setSideConversationCount((value) => Math.max(value, 1));
  }, []);

  const handleSddGenerate = useCallback(
    (prompt: string) => {
      setSddOpen(false);
      setAppMode("code");
      void handleSend(prompt);
    },
    [handleSend],
  );

  const startActiveTopicRename = useCallback(() => {
    if (!activeTab?.topicId) return;
    topicRenameSkipCommitRef.current = false;
    topicRenameCommitHandledRef.current = false;
    setRenamingTopicId(activeTab.topicId);
    setTopicTitleDraft(activeTab.topicTitle || "");
  }, [activeTab?.topicId, activeTab?.topicTitle]);

  const cancelActiveTopicRename = useCallback(() => {
    topicRenameSkipCommitRef.current = true;
    topicRenameCommitHandledRef.current = true;
    setRenamingTopicId(null);
    setTopicTitleDraft("");
  }, []);

  const commitActiveTopicRename = useCallback(async () => {
    if (topicRenameSkipCommitRef.current) {
      topicRenameSkipCommitRef.current = false;
      topicRenameCommitHandledRef.current = false;
      setRenamingTopicId(null);
      return;
    }
    if (topicRenameCommitHandledRef.current) return;
    topicRenameCommitHandledRef.current = true;
    const topicId = renamingTopicId;
    setRenamingTopicId(null);
    if (!topicId) return;
    const nextTitle = topicTitleDraft.trim();
    if (!nextTitle) return;
    await renameTopic(topicId, nextTitle);
  }, [renameTopic, renamingTopicId, topicTitleDraft]);

  const onRemember = useCallback(
    async (scope: string, note: string) => {
      await remember(scope, note);
      setMemView(await fetchMemory());
    },
    [remember, fetchMemory],
  );

  const onForget = useCallback(
    async (name: string) => {
      await forget(name);
      setMemView(await fetchMemory());
    },
    [forget, fetchMemory],
  );

  const onSaveDoc = useCallback(
    async (path: string, body: string) => {
      await saveDoc(path, body);
      setMemView(await fetchMemory());
    },
    [saveDoc, fetchMemory],
  );

  return (
    <ShellExpandProvider>
      <div className="app">
      <ShellHotkeys />
      <div
        ref={layoutRef}
        className={[
          "workbench",
          "workbench--studio",
          projectDrawerOpen ? "workbench--drawer-open" : "workbench--drawer-closed",
          workspacePanelOpen && showRightDock ? "workbench--dock-open" : "",
          !showRightDock ? "workbench--dock-hidden" : "",
          filePreviewResizing || dockResizing ? "workbench--resizing" : "",
          appMode === "write" ? "workbench--write-mode" : "",
        ]
          .filter(Boolean)
          .join(" ")}
        style={layoutStyle}
      >
        <Sidebar
          drawerOpen={projectDrawerOpen}
          onCloseDrawer={closeProjectDrawer}
          onToggleDrawer={toggleProjectDrawer}
          appMode={appMode}
          activeTab={activeTab}
          projectRevision={projectRevision}
          currentWorkspaceName={activeTab?.workspaceName || state.meta?.cwd || undefined}
          onOpenTopic={(scope, workspaceRoot, topicId) => {
            void handleOpenTopic(scope, workspaceRoot, topicId);
          }}
          onNewChat={() => {
            if (state.running) cancel();
            void startNewSession();
          }}
          onModeChange={setAppMode}
          onOpenSdd={() => setSddOpen(true)}
          onAddProject={async () => {
            await switchFolder();
          }}
          onUseCurrentProject={async () => {
            const cwd = activeTab?.workspaceRoot || state.meta?.cwd;
            if (cwd) await switchFolder(cwd);
            else await switchFolder();
          }}
          onOpenProjectHistory={(scope, workspaceRoot) => {
            void openProjectHistory(scope, workspaceRoot);
          }}
          onTopicsChanged={refreshProjectsAndTabs}
          writeConversation={writeConversationTurns}
          writeRunning={state.running}
        />

        <div
          className="workbench__main"
          style={{ "--studio-footer-band-h": `${footerHeight}px` } as CSSProperties}
        >
          {chatMode ? (
            <Topbar
              title={topicTitle(activeTab)}
              workspacePath={showTopbarWorkspace ? topbarWorkspacePath : undefined}
              editing={topicbarEditing}
              titleDraft={topicTitleDraft}
              onTitleDraftChange={setTopicTitleDraft}
              onStartRename={startActiveTopicRename}
              onCommitRename={() => void commitActiveTopicRename()}
              onCancelRename={cancelActiveTopicRename}
              running={state.running}
              goalLabel={goalLabel || undefined}
              sideConversationCount={sideConversationCount}
            />
          ) : null}

          {state.meta?.startupErr && (
            <div className="banner banner--error">{t("topbar.startupError", { msg: state.meta.startupErr })}</div>
          )}

          <UpdateBanner />

          <div
            className={[
              "workbench__body",
              filePreviewOpen ? "workbench__body--preview-open" : "",
              filePreviewExpanded ? "workbench__body--preview-expanded" : "",
              filePreviewComposerOpen ? "workbench__body--preview-composer-open" : "",
              workspacePanelRenderable ? "workbench__body--dock-open" : "",
            ]
              .filter(Boolean)
              .join(" ")}
            style={{ "--studio-dock-w": `${workspacePanelRenderWidth}px` } as CSSProperties}
          >
            <div className="workbench__stack">
              <div className="workbench__center">
                {state.meta?.ready === false && !state.meta?.startupErr ? (
                  <div className="loading-screen">
                    <div className="loading-screen__spinner" />
                    <span className="loading-screen__text">{t("common.loading")}</span>
                  </div>
                ) : chatMode ? (
                  <MessageTimeline
                    tabId={activeTabId}
                    items={deferredItems}
                    live={state.live}
                    usage={state.usage}
                    sessionCost={state.sessionCost}
                    sessionCurrency={state.sessionCurrency}
                    balance={state.balance}
                    footerHeight={footerHeight}
                    checkpoints={state.checkpoints}
                    actionPending={state.messageAction != null}
                    rewindDisabled={state.running}
                    workspaceRoot={workspaceRoot}
                    onOpenActionFile={openActionFilePreview}
                    onPrompt={handleSend}
                    onRewind={(turn, scope) => void rewind(turn, scope)}
                  />
                ) : (
                  <ModeWorkspaceCenter
                    mode={appMode}
                    workspaceRoot={workspaceRoot}
                    activeTabId={activeTabId}
                    activeTabLabel={activeTab?.topicTitle?.trim() || topicTitle(activeTab)}
                    activeWorkspaceName={activeTab?.workspaceName}
                    writeWorkspaceRoot={writeWorkspaceRoot}
                    onWriteWorkspaceChange={handleWriteWorkspaceChange}
                    onPrompt={handleSend}
                    onDraftComposer={addWriteContextToComposer}
                    onPickWriteWorkspace={pickWriteWorkspace}
                    onFilesChanged={() => setProjectRevision((value) => value + 1)}
                    writeConversation={writeConversationTurns}
                    writeAgentRunning={state.running}
                    onSettingsChanged={() => void refreshMeta()}
                    onOpenHistory={() => {
                      setAppMode("code");
                      void openAllHistory();
                    }}
                    onOpenMemory={() => {
                      setAppMode("code");
                      void openMemory();
                    }}
                    onOpenCapabilities={() => {
                      setAppMode("code");
                      setCapsOpen(true);
                    }}
                    onOpenTrash={() => {
                      setAppMode("code");
                      void openTrash();
                    }}
                    onConfigureProjectSandbox={() => setSandboxSetup({ reason: "manual" })}
                    onModeChange={(mode) => {
                      setAppMode(mode);
                    }}
                    onOpenDockTab={(tab) => {
                      setAppMode("code");
                      openDockTab(tab, { toggle: false });
                    }}
                    onOpenTerminal={() => {
                      setAppMode("code");
                      setTerminalOpen(true);
                    }}
                  />
                )}
              </div>

              {showWorkbenchFooter ? (
                <div className="workbench__footer" ref={footerRef}>
                  {chatMode && state.approval && !filePreviewComposerOpen && (
                    <ApprovalModal
                      approval={state.approval}
                      onAnswer={(allow, session, persist) => {
                        if (state.approval!.tool === "exit_plan_mode" && allow) applyMode("normal");
                        approve(state.approval!.id, allow, session, persist);
                      }}
                      onRevisePlan={(text) => {
                        setPendingPlanRevision(text);
                        approve(state.approval!.id, false, false, false);
                      }}
                      onExitPlan={() => {
                        applyMode("normal");
                        approve(state.approval!.id, false, false, false);
                      }}
                    />
                  )}
                  {chatMode && state.ask && !filePreviewComposerOpen && (
                    <AskCard
                      ask={state.ask}
                      onAnswer={answerQuestion}
                      onDismiss={() => answerQuestion(state.ask!.id, [])}
                    />
                  )}
                  <div className={`workbench__footer-stack${terminalOpen ? " workbench__footer-stack--terminal-open" : ""}`}>
                    <div className="workbench__composer-zone">
                      <FloatingComposer
                        key={appMode === "write" ? "write" : "code"}
                        composerSurface={appMode === "write" ? "write" : "code"}
                        running={state.running}
                        mode={mode}
                        cwd={composerCwd}
                        modelLabel={state.meta?.label ?? t("status.connecting")}
                        tabId={activeTabId}
                        effort={state.effort}
                        onSend={handleSend}
                        onCancel={cancel}
                        onCycleMode={cycleMode}
                        onSetMode={applyMode}
                        onSwitchModel={switchModel}
                        onSetEffort={setEffort}
                        onPickFolder={composerPickFolder}
                        onRemoveWorkspace={removeWorkspace}
                        insertRequest={composerInsertRequest}
                        disabled={state.meta?.ready === false || state.approval != null || state.ask != null}
                        decisionPending={state.approval != null || state.ask != null}
                        ready={state.meta?.ready === true}
                        turnStartAt={state.turnStartAt}
                        turnTokens={state.turnTokens}
                        retry={state.retry}
                        workspaceRefreshSignal={projectRevision}
                        showWorkspaceSwitcher
                        workspaceNone={composerWorkspaceNone}
                        onUseNoWorkspace={handleUseNoWorkspace}
                        terminalActive={chatMode && terminalOpen && terminalTabs.length > 0}
                        terminalLabel={activeTerminalTab?.title}
                        context={state.context}
                        usage={state.usage}
                        balance={state.balance}
                        sessionCost={state.sessionCost}
                        sessionCurrency={state.sessionCurrency}
                        terminalCount={terminalOpen ? terminalTabs.length : 0}
                      />
                    </div>
                    {terminalOpen && !filePreviewComposerOpen && terminalTabs.length > 0 && resolvedActiveTerminalId && (
                      <BottomTerminalPanel
                        height={terminalHeight}
                        cwd={composerCwd}
                        tabs={terminalTabs}
                        activeId={resolvedActiveTerminalId}
                        onActiveChange={setActiveTerminalId}
                        onNewTerminal={() => void openNewTerminal()}
                        onCloseTab={closeTerminalTab}
                        onClosePanel={closeTerminalPanel}
                        onResizeHeight={setSavedTerminalHeight}
                      />
                    )}
                  </div>
                </div>
              ) : null}
            </div>

            {filePreviewOpen && filePreviewPath && (
              <>
                <button
                  className="workbench__resizer workbench__resizer--preview wails-no-drag"
                  type="button"
                  role="separator"
                  aria-orientation="vertical"
                  aria-label={t("filePreview.resize")}
                  onPointerDown={startFilePreviewResize}
                  onKeyDown={resizeFilePreviewWithKeyboard}
                  onDoubleClick={() => setSavedFilePreviewWidth(clampFilePreviewWidth(rightDockWidth))}
                />
                <FilePreviewPanel
                  path={filePreviewPath}
                  diff={filePreviewDiff}
                  expanded={filePreviewExpanded}
                  onToggleExpanded={toggleFilePreviewExpanded}
                  onClose={closeFilePreview}
                  onAddToChat={addWorkspaceTextToComposer}
                />
              </>
            )}

            {showRightDock && workspacePanelRenderable && (
              <>
                <button
                  className="workbench__resizer workbench__resizer--dock wails-no-drag"
                  type="button"
                  role="separator"
                  aria-orientation="vertical"
                  aria-label={t("rightDock.resize")}
                  onPointerDown={startDockResize}
                />
                <RightDock
                  open={workspacePanelRenderable}
                  tab={rightDockMode}
                  onTabChange={(tab) => openDockTab(tab, { toggle: false })}
                  onClose={closeWorkspacePanel}
                  tabId={activeTabId}
                  context={state.context}
                  usage={state.usage}
                  sessionCost={state.sessionCost}
                  sessionCurrency={state.sessionCurrency}
                  scopeLabel={topicScopeLabel(activeTab)}
                  refreshKey={projectRevision}
                  modelLabel={state.meta?.label}
                  mode={mode}
                  effort={state.effort}
                  balance={state.balance}
                  running={state.running}
                  cwd={composerCwd}
                  onAddToChat={addWorkspaceTextToComposer}
                  filePreviewPath={filePreviewPath}
                  onOpenFile={(path, dockTab) => openFilePreview(path, dockTab ?? "files")}
                  todos={showTodos ? todos : []}
                  todoStale={todoStale}
                  onDismissTodos={() => setDismissedTodo(todoItem!.id)}
                  onStartPlan={() => handleSend("/plan")}
                  codeReview={codeReview}
                  onRunCodeReview={runCodeReview}
                  onClearCodeReview={clearCodeReview}
                  browserExpanded={browserPreviewExpanded}
                  onToggleBrowserExpanded={toggleBrowserPreviewExpanded}
                />
              </>
            )}

            {(chatMode || appMode === "write") && (
              <StudioToolRail
                dockOpen={workspacePanelOpen}
                activeDockTab={workspacePanelOpen ? rightDockMode : null}
                terminalOpen={terminalOpen}
                onHubPress={openDockHub}
                onOpenDockTab={(tab) => openDockTab(tab, { toggle: false })}
                onOpenPreviewMode={togglePreviewMode}
              />
            )}
          </div>
        </div>
      </div>

      {memView !== null && (
        <MemoryPanel
          view={memView}
          onClose={closeMemory}
          onRemember={onRemember}
          onForget={onForget}
          onSaveDoc={onSaveDoc}
        />
      )}

      {histView !== null && (
        <HistoryPanel
          kind={histView.kind}
          sessions={histView.sessions}
          running={state.running}
          onResume={onResumeSession}
          onPreview={previewSession}
          onDelete={onDeleteSession}
          onRename={onRenameSession}
          onRestore={onRestoreTrashedSession}
          onPurge={onPurgeTrashedSession}
          onPurgeAll={onPurgeAllTrashedSessions}
          onClose={closeHistory}
        />
      )}

      {capsOpen && <CapabilitiesPanel onClose={() => setCapsOpen(false)} />}
      {needsOnboarding && <OnboardingOverlay onComplete={() => setNeedsOnboarding(false)} />}
      {sandboxSetup && (
        <SandboxSetupOverlay
          reason={sandboxSetup.reason}
          onCancel={() => {
            pendingYoloRef.current = false;
            setSandboxSetup(null);
          }}
          onComplete={() => {
            setSandboxSetup(null);
            if (pendingYoloRef.current) {
              pendingYoloRef.current = false;
              setMode("yolo");
              void syncModeToController("yolo");
            }
          }}
        />
      )}
      {sddOpen && (
        <RequirementDraft
          onClose={() => setSddOpen(false)}
          onGeneratePlan={handleSddGenerate}
          onAiAssist={(stepText) => handleSend(t("sdd.aiAssistPrompt", { text: stepText }))}
        />
      )}
      <SideConversation
        mainTitle={topicTitle(activeTab)}
        messages={sideMessages}
        onSend={handleSideSend}
        onClose={() => {
          setSideMessages([]);
          setSideConversationCount(0);
        }}
      />
      </div>
    </ShellExpandProvider>
  );
}
