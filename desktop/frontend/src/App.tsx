import { lazy, Suspense, useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { ShellExpandProvider, useShellExpand } from "./lib/shellExpand";
import { clearLegacyLangPref, normalizeLangPref, readLegacyLangPref, t, useI18n, useT } from "./lib/i18n";
import { useController } from "./lib/useController";
import { app, onProjectTreeChanged, onScheduleTask } from "./lib/bridge";
import { logBridgeError } from "./lib/logBridgeError";
import { toErrorMessage } from "./lib/errors";
import { IPC_ONBOARDING_TIMEOUT_MS, withIPCTimeout } from "./lib/ipc";
import { findDevPreviewTrigger, isCancellableAgentWork } from "./lib/agentActivity";
import { isComposerSendDisabled } from "./lib/composerSendGate";
import { useRuntimeReady } from "./lib/runtime";
import { MessageTimeline } from "./components/MessageTimeline";
import { FloatingComposer } from "./components/FloatingComposer";
import { TurnProgressLine } from "./components/TurnProgressLine";
import { BottomTerminalPanel } from "./components/TerminalPanel";
import type { CodeReviewState } from "./components/CodeReviewSection";
import type { ReviewMode, ReviewScope } from "./lib/codeReview";
import { Sidebar } from "./components/Sidebar";
import { OpenTabsBar } from "./components/OpenTabsBar";
import { Topbar } from "./components/Topbar";
import { countBackgroundAttention, listTabAttention, openTabsBarItems } from "./lib/tabSessionActivity";
import { StudioToolRail } from "./components/StudioToolRail";
import { RightDock } from "./components/RightDock";
import type { RightDockTab } from "./components/Topbar";
import { SettingsDockModal } from "./components/SettingsDockModal";
import { SettingsWorkspaceDataModal, type SettingsDataModalState } from "./components/SettingsWorkspaceDataModal";
import { FilePreviewPanel } from "./components/FilePreviewPanel";
import { AgentDecisionLayer } from "./components/AgentDecisionLayer";
import { clearAgentDecisionNotifications, notifyAgentDecision } from "./lib/agentNotifications";
const HistoryPanel = lazy(() => import("./components/HistoryPanel").then((m) => ({ default: m.HistoryPanel })));
const LazyMemoryPanel = lazy(() => import("./components/MemoryPanel").then((m) => ({ default: m.MemoryPanel })));
import { UpdateBanner } from "./components/UpdateBanner";
import { ConnectionRecoveryBanner } from "./components/ConnectionRecoveryBanner";
import { OnboardingOverlay } from "./components/OnboardingOverlay";
import { SandboxSetupOverlay } from "./components/SandboxSetupOverlay";
import { SideConversation, type SideMessage } from "./components/SideConversation";
import { RequirementDraft } from "./components/RequirementDraft";
import { ModeWorkspaceCenter } from "./components/ModeWorkspaceCenter";
import { DevMockBanner } from "./components/DevMockBanner";
import { buildWriteConversation } from "./lib/writeConversation";
import { useWriteModeTab } from "./lib/useWriteModeTab";
import type { AppMode } from "./lib/appMode";
import { getDefaultAppMode } from "./lib/startupPrefs";
import { parseTodos } from "./lib/tools";
import { shouldShowTodoPanel } from "./lib/todoVisibility";
import type { ComposerInsertRequest, ComposerWriteContext, MemoryView, Mode, QuestionAnswer, SessionMeta, TabMeta } from "./lib/types";
import { recordRecentWorkspace } from "./lib/workspaceRecents";
import {
  clearStoredCodeWorkspaceRoot,
  getStoredCodeWorkspaceRoot,
  getStoredComposerNoWorkspace,
  isUsableCodeWorkspaceRoot,
  sameWorkspaceRoot,
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
import { applyThemeFromSettings } from "./lib/applyThemeFromSettings";
import {
  clearLegacyThemePreference,
  normalizeThemePreference,
  normalizeThemeStyleForTheme,
  readLegacyThemePreference,
} from "./lib/theme";
import {
  markLocalAppearanceMigrated,
  readLocalAppearanceForMigration,
} from "./lib/appearancePrefs";
import {
  markLocalCodeReviewMigrated,
  readLocalCodeReviewForMigration,
} from "./lib/codeReviewPrefs";
import { GITHUB_CLI_SETTINGS_EVENT } from "./lib/gitHubCliSettingsNav";
import { useWindowStatePersistence } from "./lib/windowState";
import { useDesktopSendRouter } from "./lib/useDesktopSendRouter";
import { useTabMetas } from "./lib/useTabMetas";
import { useProjectDrawer } from "./lib/useProjectDrawer";
import { useBrowserPanel } from "./lib/useBrowserPanel";
import { useTerminalPanel } from "./lib/useTerminalPanel";
import { useWorkbenchDock } from "./lib/useWorkbenchDock";

type HistoryScopeFilter = { scope: "global" | "project"; workspaceRoot: string };
type HistoryViewState =
  | { kind: "history"; source: "scope"; filter: HistoryScopeFilter; sessions: SessionMeta[] }
  | { kind: "history"; source: "all"; sessions: SessionMeta[] }
  | { kind: "trash"; sessions: SessionMeta[] };

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
    switchTab,
    closeTab,
    syncActiveTab,
    rewind,
    bootPhase,
    getAllTabStates,
    rememberTabTitles,
  } = useController();
  const runtimeReady = useRuntimeReady();
  const { locale, setPref: setLocalePref } = useI18n();
  const t = useT();
  const [modesByTab, setModesByTab] = useState<Record<string, Mode>>({});
  const [composerModePref, setComposerModePref] = useState<Mode>("normal");
  const { tabMetas, refreshTabMetas } = useTabMetas();
  useEffect(() => {
    rememberTabTitles(tabMetas);
  }, [rememberTabTitles, tabMetas]);

  const tabAttention = useMemo(
    () =>
      listTabAttention(tabMetas, getAllTabStates(), {
        plan: t("decision.pendingPlan"),
        approval: (tool) => t("decision.pendingApproval", { tool }),
        ask: t("decision.pendingAsk"),
      }),
    [getAllTabStates, state, tabMetas, t],
  );
  const openTabs = useMemo(() => openTabsBarItems(tabAttention), [tabAttention]);
  const backgroundAttentionCount = useMemo(
    () => countBackgroundAttention(tabAttention, activeTabId),
    [activeTabId, tabAttention],
  );
  const { projectDrawerOpen, setProjectDrawerOpen, closeProjectDrawer, toggleProjectDrawer } = useProjectDrawer();
  // null until the mount probe resolves; true shows the overlay. Probed once —
  // clearing the key mid-session is the Settings panel's job, not the gate's.
  const [onboardingGate, setOnboardingGate] = useState<boolean | null>(null);
  const [onboardingManual, setOnboardingManual] = useState(false);
  const [onboardingSession, setOnboardingSession] = useState(0);
  const [sandboxSetup, setSandboxSetup] = useState<null | { reason: "yolo" | "manual" }>(null);
  const [memView, setMemView] = useState<MemoryView | null>(null);
  const [histView, setHistView] = useState<HistoryViewState | null>(null);
  const [settingsDockTab, setSettingsDockTab] = useState<RightDockTab | null>(null);
  const [settingsDataModal, setSettingsDataModal] = useState<SettingsDataModalState | null>(null);
  const [projectRevision, setProjectRevision] = useState(0);
  const [composerInsertRequest, setComposerInsertRequest] = useState<ComposerInsertRequest | null>(null);
  const [appMode, setAppMode] = useState<AppMode>(() => getDefaultAppMode());
  const [writeWorkspaceRoot, setWriteWorkspaceRoot] = useState(() => getInitialWriteWorkspaceRoot());
  const [composerNoWorkspace, setComposerNoWorkspace] = useState(() => getStoredComposerNoWorkspace());
  const [sddOpen, setSddOpen] = useState(false);
  const [goalLabel, setGoalLabel] = useState<string>("");
  const [sideConversationCount, setSideConversationCount] = useState(0);
  const [sideMessages, setSideMessages] = useState<SideMessage[]>([]);
  const [sideChatBusy, setSideChatBusy] = useState(false);
  const dispatchSideChatRef = useRef<(text: string) => Promise<void>>(async () => {});
  const [codeReview, setCodeReview] = useState<CodeReviewState>({ status: "idle", mode: "standard", scope: "all" });
  const [renamingTopicId, setRenamingTopicId] = useState<string | null>(null);
  const [topicTitleDraft, setTopicTitleDraft] = useState("");
  const topicRenameSkipCommitRef = useRef(false);
  const topicRenameCommitHandledRef = useRef(false);
  const codeWorkspaceRestoredRef = useRef(false);
  const prevAppModeRef = useRef<AppMode>(appMode);
  const {
    terminalHasSessions,
    terminalPanelVisible,
    terminalTabs,
    terminalPanelShown,
    terminalAnimHeight,
    terminalMotionKey,
    setActiveTerminalId,
    resolvedActiveTerminalId,
    openNewTerminal,
    closeTerminalPanel,
    minimizeTerminalPanel,
    restoreTerminalPanel,
    closeTerminalTab,
    setSavedTerminalHeight,
  } = useTerminalPanel({ notice });
  const {
    browserTabs,
    browserActive,
    activeBrowserTabId,
    setActiveBrowserTabId,
    openBrowserTab,
    updateBrowserTab,
    closeBrowserTab,
    closeAllBrowserTabs,
  } = useBrowserPanel();
  const {
    layoutRef,
    layoutStyle,
    workspacePanelOpen,
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
    browserPreviewExpanded,
    pagePreviewActive,
    dockMounted,
    dockBackgroundSessions,
    pagePreviewPath,
    setPagePreviewPath,
    openWebPreview,
    openPagePreview,
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
    resizeDockWithKeyboard,
    resetFilePreviewWidthFromDock,
    resetDockWidthFromDefault,
    clearFilePreviewComposerOpen,
    addWorkspaceTextToComposer,
  } = useWorkbenchDock({
    appMode,
    projectDrawerOpen,
    browserActive,
    openBrowserTab,
    closeAllBrowserTabs,
    terminalHasSessions,
    terminalPanelVisible,
    minimizeTerminalPanel,
    restoreTerminalPanel,
    closeTerminalPanel,
    openNewTerminal,
    cwd: state.meta?.cwd,
    setComposerInsertRequest,
  });

  const handleCloseBrowserTab = useCallback(
    (id: string) => {
      closeBrowserTab(id);
    },
    [closeBrowserTab],
  );

  useEffect(() => {
    if (browserActive) return;
    if (rightDockMode !== "browser") return;
    if (!workspacePanelOpen) return;
    closeWorkspacePanel();
  }, [browserActive, closeWorkspacePanel, rightDockMode, workspacePanelOpen]);

  // Persist window geometry across launches.
  useWindowStatePersistence();

  useEffect(() => {
    void app.Platform()
      .then((platform) => {
        if (platform === "darwin") {
          document.documentElement.setAttribute("data-platform", "darwin");
        }
      })
      .catch(() => {
        /* platform hint is optional */
      });
  }, []);

  useEffect(() => {
    let cancelled = false;
    const syncDesktopPreferences = async () => {
      const legacyLanguage = readLegacyLangPref();
      const legacyTheme = readLegacyThemePreference();
      if (legacyLanguage || legacyTheme.hasValue) {
        const migrateTheme = normalizeThemePreference(legacyTheme.theme);
        let migrateStyle = normalizeThemeStyleForTheme(legacyTheme.style, migrateTheme);
        if (migrateTheme !== "dark" && migrateStyle !== "glacier") {
          migrateStyle = "glacier";
        }
        await app.MigrateDesktopPreferences(legacyLanguage, legacyTheme.theme, migrateStyle);
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
      applyThemeFromSettings(settings, "boot");
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
    if (typeof window === "undefined" || !window.runtime) return;
    return window.runtime.EventsOn("app:open-web-preview", (payload) => {
      const detail = payload as { url?: string } | undefined;
      const url = typeof detail?.url === "string" ? detail.url : undefined;
      setAppMode("code");
      void (async () => {
        if (!url) {
          try {
            const detected = await app.DetectDevServerURL();
            openWebPreview(detected || undefined);
            return;
          } catch {
            openWebPreview();
            return;
          }
        }
        openWebPreview(url);
      })();
    });
  }, [openWebPreview]);

  useEffect(() => {
    const openGitHubCliSettings = () => setAppMode("settings");
    window.addEventListener(GITHUB_CLI_SETTINGS_EVENT, openGitHubCliSettings);
    return () => window.removeEventListener(GITHUB_CLI_SETTINGS_EVENT, openGitHubCliSettings);
  }, []);
  const [pendingPlanRevision, setPendingPlanRevision] = useState<string | null>(null);
  const [footerHeight, setFooterHeight] = useState(0);
  const footerRef = useRef<HTMLDivElement>(null);
  const chatMode = appMode === "code";
  const showWorkbenchFooter = chatMode || appMode === "write";
  const devPreviewTriggerRef = useRef<string | null>(null);

  useEffect(() => {
    if (!chatMode) return;
    const trigger = findDevPreviewTrigger(state.items);
    if (!trigger) return;
    const key = JSON.stringify(trigger);
    if (devPreviewTriggerRef.current === key) return;
    devPreviewTriggerRef.current = key;
    void (async () => {
      if (trigger.url) {
        openWebPreview(trigger.url);
        return;
      }
      try {
        const detected = await app.DetectDevServerURL();
        openWebPreview(detected || undefined);
      } catch {
        openWebPreview();
      }
    })();
  }, [chatMode, openWebPreview, state.items]);

  useEffect(() => {
    if (chatMode) return;
    setRenamingTopicId(null);
    setTopicTitleDraft("");
  }, [chatMode]);

  useEffect(() => {
    if (appMode !== "write") return;
    void app.EnsureBundledSkills().catch((err) => logBridgeError("EnsureBundledSkills", err));
  }, [appMode]);

  useEffect(() => {
    setComposerInsertRequest(null);
  }, [appMode]);

  const activeTab = useMemo(
    () => tabMetas.find((tab) => tab.id === activeTabId) ?? tabMetas.find((tab) => tab.active),
    [activeTabId, tabMetas],
  );
  const { activateWriteTab, restoreCodeTab, ensureWriteTabMatchesWorkspace } = useWriteModeTab({
    appMode,
    writeWorkspaceRoot,
    activeTabId,
    activeTab,
    tabMetas,
    openProjectTab,
    openGlobalTab,
    switchTab,
    syncActiveTab,
    refreshTabMetas,
  });
  const mode = composerModePref;
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
      const next = { ...current };
      for (const tab of tabMetas) {
        if (tab.id in next) continue;
        next[tab.id] = normalizeModeValue(tab.mode);
        changed = true;
      }
      for (const id of Object.keys(next)) {
        if (!ids.has(id)) {
          delete next[id];
          changed = true;
        }
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
    void app.SetTrayLocale(locale).catch((err) => logBridgeError("SetTrayLocale", err));
  }, [locale]);

  // applyMode is the single source of truth for the input mode: it updates the
  // local pill and pushes the matching gate state to the controller (plan = read
  // only; yolo = auto-approve every tool call). normal clears both.
  const applyMode = useCallback(
    (m: Mode) => {
      setComposerModePref(m);
      if (activeTabId) setMode(m);
      void syncModeToController(m);
    },
    [activeTabId, setMode, syncModeToController],
  );

  const lastModeTabRef = useRef<string | undefined>();
  useEffect(() => {
    if (!activeTabId || lastModeTabRef.current === activeTabId) return;
    lastModeTabRef.current = activeTabId;
    const tabMode = modesByTab[activeTabId] ?? normalizeModeValue(activeTab?.mode);
    setComposerModePref(tabMode);
    void syncModeToController(tabMode);
  }, [activeTab?.mode, activeTabId, modesByTab, syncModeToController]);
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
  const deferredLive = useDeferredValue(state.live);
  const showBootLoading =
    !activeTabId &&
    state.meta?.ready === false &&
    !state.meta?.startupErr &&
    state.items.length === 0 &&
    !state.pendingUser;
  const bootLoadingText = bootPhase ?? t("common.loading");

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
  const handleSend = useDesktopSendRouter({
    appMode,
    mode,
    filePreviewComposerOpen,
    t,
    notice,
    runShell,
    switchModel,
    openMemory,
    setGoalLabel,
    dispatchSideChat: (text) => dispatchSideChatRef.current(text),
    setAppMode,
    openDockTab,
    openWebPreview,
    runCodeReview,
    setSddOpen,
    syncModeToController,
    send,
    exitExpandedPreviewComposer,
  });

  useEffect(() => {
    return onProjectTreeChanged(() => {
      setProjectRevision((value) => value + 1);
    });
  }, []);

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
    const fallback = window.setTimeout(() => {
      if (!cancelled) setOnboardingGate(false);
    }, IPC_ONBOARDING_TIMEOUT_MS);
    (async () => {
      try {
        const needs = await withIPCTimeout(
          app.NeedsOnboarding(),
          IPC_ONBOARDING_TIMEOUT_MS,
          "NeedsOnboarding",
        );
        if (!cancelled) setOnboardingGate(needs);
      } catch {
        if (!cancelled) setOnboardingGate(false);
      } finally {
        window.clearTimeout(fallback);
      }
    })();
    return () => {
      cancelled = true;
      window.clearTimeout(fallback);
    };
  }, []);

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
  }, [filePreviewComposerOpen, filePreviewExpanded, chatMode, appMode, terminalPanelVisible, state.approval, state.ask, showWorkbenchFooter]);

  const toggleTerminal = useCallback(() => {
    togglePreviewTerminal();
  }, [togglePreviewTerminal]);

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

  const addWriteContextToComposer = useCallback((context: ComposerWriteContext) => {
    if (appMode !== "write") return;
    setComposerInsertRequest({
      id: Date.now(),
      writeContext: context,
      replace: true,
    });
    clearFilePreviewComposerOpen();
  }, [appMode, clearFilePreviewComposerOpen]);

  const pickWriteWorkspace = useCallback(async () => {
    const picked = await pickWorkspace();
    if (picked) {
      setComposerNoWorkspace(false);
      setStoredComposerNoWorkspace(false);
      setStoredWriteWorkspaceRoot(picked);
      recordRecentWorkspace(picked);
      setWriteWorkspaceRoot(picked);
      await app.AddWriteWorkspace(picked).catch(() => undefined);
      if (appMode === "write") await activateWriteTab(picked);
    }
    return picked || undefined;
  }, [appMode, activateWriteTab, pickWorkspace]);

  const handleWriteWorkspaceChange = useCallback((root: string) => {
    if (isNoWriteWorkspace(root)) {
      setStoredWriteWorkspaceRoot(NO_WORKSPACE_VALUE);
      setWriteWorkspaceRoot(NO_WORKSPACE_VALUE);
      setComposerNoWorkspace(true);
      setStoredComposerNoWorkspace(true);
      if (appMode === "write") void ensureWriteTabMatchesWorkspace(NO_WORKSPACE_VALUE);
      return;
    }
    if (!isUsableWriteWorkspaceRoot(root)) return;
    setComposerNoWorkspace(false);
    setStoredComposerNoWorkspace(false);
    setStoredWriteWorkspaceRoot(root);
    recordRecentWorkspace(root);
    setWriteWorkspaceRoot(root);
    if (appMode === "write") void ensureWriteTabMatchesWorkspace(root);
  }, [appMode, ensureWriteTabMatchesWorkspace]);

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

  const handleOpenTopic = useCallback(async (scope: string, workspaceRoot: string, topicId: string, freshSession = false) => {
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
        ? await openGlobalTab(trimmedTopicId, false, freshSession)
        : await openProjectTab(workspaceRoot, trimmedTopicId, false, freshSession);
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
      setSettingsDataModal(null);
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
      setSettingsDataModal((cur) => (cur?.kind === "history" ? { kind: "history", sessions } : cur));
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
      setSettingsDataModal((cur) => (cur?.kind === "history" ? { kind: "history", sessions } : cur));
    },
    [state.running, renameSession, listSessions],
  );
  const onRestoreTrashedSession = useCallback(
    async (path: string) => {
      await restoreSession(path);
      const trashed = await listTrashedSessions();
      setHistView((cur) => (cur === null ? null : { kind: "trash", sessions: trashed }));
      setSettingsDataModal((cur) => (cur?.kind === "trash" ? { kind: "trash", sessions: trashed } : cur));
    },
    [restoreSession, listTrashedSessions],
  );
  const onPurgeTrashedSession = useCallback(
    async (path: string) => {
      await purgeTrashedSession(path);
      const trashed = await listTrashedSessions();
      setHistView((cur) => (cur === null ? null : { kind: "trash", sessions: trashed }));
      setSettingsDataModal((cur) => (cur?.kind === "trash" ? { kind: "trash", sessions: trashed } : cur));
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
      setSettingsDataModal((cur) => (cur?.kind === "trash" ? { kind: "trash", sessions: trashed } : cur));
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

  const openCodeWorkspace = useCallback(async () => {
    setAppMode("code");
    return switchFolder();
  }, [switchFolder]);

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
    codeWorkspaceRestoredRef.current = true;
    void syncActiveTab(false);
  }, [state.meta?.ready, syncActiveTab]);

  useEffect(() => {
    const prev = prevAppModeRef.current;
    prevAppModeRef.current = appMode;
    if (prev === "settings" && appMode !== "settings") {
      setSettingsDockTab(null);
      setSettingsDataModal(null);
    }
    if (appMode === "write" && prev !== "write") {
      const stored = getStoredWriteWorkspaceRoot();
      let nextRoot = writeWorkspaceRoot;
      if (isNoWriteWorkspace(stored)) {
        nextRoot = NO_WORKSPACE_VALUE;
        setWriteWorkspaceRoot(NO_WORKSPACE_VALUE);
      } else if (isUsableWriteWorkspaceRoot(stored)) {
        nextRoot = stored;
        setWriteWorkspaceRoot(stored);
      }
      void activateWriteTab(nextRoot);
      return;
    }
    if (prev === "write" && appMode !== "write") {
      void restoreCodeTab();
    }
    if (appMode === "code" && prev !== "code" && !composerNoWorkspace) {
      const stored = getStoredCodeWorkspaceRoot();
      const activeRoot = activeTab?.workspaceRoot?.trim() ?? "";
      if (isUsableCodeWorkspaceRoot(stored) && (!activeRoot || !sameWorkspaceRoot(activeRoot, stored))) {
        void switchFolder(stored);
      }
    }
  }, [appMode, composerNoWorkspace, switchFolder, activeTab?.workspaceRoot, activateWriteTab, restoreCodeTab, writeWorkspaceRoot]);

  useEffect(() => {
    if (appMode !== "write" || state.meta?.ready !== true) return;
    void ensureWriteTabMatchesWorkspace(writeWorkspaceRoot);
  }, [appMode, ensureWriteTabMatchesWorkspace, state.meta?.ready, writeWorkspaceRoot]);

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

  const startNewTopic = useCallback(async (freshSession = false) => {
    setAppMode("code");
    let scope: "global" | "project" = activeTab?.scope === "global" ? "global" : "project";
    let workspaceRoot = scope === "global" ? "" : (activeTab?.workspaceRoot?.trim() || getStoredCodeWorkspaceRoot() || "");
    if (scope === "project" && !isUsableCodeWorkspaceRoot(workspaceRoot)) {
      const picked = await switchFolder();
      if (!picked) return;
      workspaceRoot = picked;
      scope = "project";
      await refreshTabMetas();
    }
    if (state.meta?.ready !== true) {
      notice(t("sidebar.newSessionNotReady"), "warn");
      return;
    }
    const continuity = freshSession ? "fresh" : "continue";
    try {
      await app.CreateTopic(scope, workspaceRoot, "", continuity);
      setProjectDrawerOpen(true);
      await refreshProjectsAndTabs();
    } catch {
      notice(t("sidebar.newSessionNotReady"), "warn");
    }
  }, [
    activeTab?.scope,
    activeTab?.workspaceRoot,
    handleOpenTopic,
    notice,
    refreshProjectsAndTabs,
    refreshTabMetas,
    setProjectDrawerOpen,
    state.meta?.ready,
    switchFolder,
    t,
  ]);

  const startNewSession = useCallback(() => startNewTopic(false), [startNewTopic]);
  const startFreshSession = useCallback(() => startNewTopic(true), [startNewTopic]);

  const handleCloseOpenTab = useCallback(
    async (tabId: string) => {
      const row = tabAttention.find((tab) => tab.tabId === tabId);
      if (row?.running && !window.confirm(t("openTabs.closeRunningConfirm"))) return;
      await closeTab(tabId);
      await refreshTabMetas();
    },
    [closeTab, refreshTabMetas, t, tabAttention],
  );

  const focusBackgroundSession = useCallback(() => {
    const next = tabAttention.find(
      (tab) => tab.tabId !== activeTabId && (tab.needsDecision || tab.running),
    );
    if (next) void switchTab(next.tabId);
  }, [activeTabId, switchTab, tabAttention]);

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

  const handleSideSend = useCallback(async (text: string) => {
    const trimmed = text.trim();
    if (!trimmed || sideChatBusy) return;
    const userId = `side-${Date.now()}`;
    const pendingId = `side-pending-${Date.now()}`;
    setSideMessages((current) => [
      ...current,
      { id: userId, text: trimmed, outgoing: true, createdAt: Date.now() },
      { id: pendingId, text: t("sideChat.thinking"), outgoing: false, createdAt: Date.now(), pending: true },
    ]);
    setSideConversationCount((value) => Math.max(value, 1));
    setSideChatBusy(true);
    try {
      const reply = await app.SideChatReply(trimmed);
      setSideMessages((current) =>
        current
          .filter((message) => message.id !== pendingId)
          .concat({ id: `side-reply-${Date.now()}`, text: reply, outgoing: false, createdAt: Date.now() }),
      );
    } catch (e) {
      setSideMessages((current) =>
        current
          .filter((message) => message.id !== pendingId)
          .concat({
            id: `side-err-${Date.now()}`,
            text: t("common.operationFailed", { msg: toErrorMessage(e) }),
            outgoing: false,
            createdAt: Date.now(),
          }),
      );
    } finally {
      setSideChatBusy(false);
    }
  }, [sideChatBusy, t]);

  dispatchSideChatRef.current = handleSideSend;

  const decisionPending = state.approval != null || state.ask != null;

  const pendingDecisionLabel = useMemo(() => {
    if (state.approval) {
      if (state.approval.tool === "exit_plan_mode") return t("decision.pendingPlan");
      return t("decision.pendingApproval", { tool: state.approval.tool });
    }
    if (state.ask) return t("decision.pendingAsk");
    return undefined;
  }, [state.approval, state.ask, t]);

  const planToolCount = useMemo(() => {
    if (state.approval?.tool !== "exit_plan_mode") return undefined;
    return state.items.filter((item) => item.kind === "tool" && item.readOnly === false).length;
  }, [state.approval, state.items]);

  const focusPendingDecision = useCallback(() => {
    document.querySelector(".workbench__composer-zone .arc-decision-layer")?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, []);

  const agentActive = state.running || state.turnActive || state.approval != null || state.ask != null;
  const composerAgentRunning = isCancellableAgentWork(state);

  useEffect(() => {
    if (!decisionPending) {
      clearAgentDecisionNotifications();
      return;
    }
    if (typeof document !== "undefined" && document.hasFocus()) return;
    notifyAgentDecision(state.approval, state.ask, {
      approvalTitle: t("decision.notifyApproval"),
      askTitle: t("decision.notifyAsk"),
      bodyApproval: (tool) => t("decision.pendingApproval", { tool }),
      bodyAsk: t("decision.pendingAsk"),
    });
  }, [decisionPending, state.approval, state.ask, t]);

  const handleAnswerQuestion = useCallback(
    (id: string, answers: QuestionAnswer[]) => {
      clearAgentDecisionNotifications();
      answerQuestion(id, answers);
    },
    [answerQuestion],
  );

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
      const view = await fetchMemory();
      setMemView(view);
      setSettingsDataModal((cur) => (cur?.kind === "memory" ? { kind: "memory", view } : cur));
    },
    [remember, fetchMemory],
  );

  const onForget = useCallback(
    async (name: string) => {
      await forget(name);
      const view = await fetchMemory();
      setMemView(view);
      setSettingsDataModal((cur) => (cur?.kind === "memory" ? { kind: "memory", view } : cur));
    },
    [forget, fetchMemory],
  );

  const onSaveDoc = useCallback(
    async (path: string, body: string) => {
      await saveDoc(path, body);
      const view = await fetchMemory();
      setMemView(view);
      setSettingsDataModal((cur) => (cur?.kind === "memory" ? { kind: "memory", view } : cur));
    },
    [saveDoc, fetchMemory],
  );

  const openOnboardingManual = useCallback(() => {
    setOnboardingSession((session) => session + 1);
    setOnboardingManual(true);
  }, []);

  const showConnectionRecovery = Boolean(
    state.meta?.startupErr && !onboardingManual && onboardingGate !== true,
  );

  const onboardingProbePending = onboardingGate === null && !onboardingManual;

  return (
    <ShellExpandProvider>
      <div className="app">
      <ShellHotkeys />
      {onboardingProbePending ? (
        <div className="loading-screen loading-screen--gate">
          <div className="loading-screen__spinner" />
          <span className="loading-screen__text">{t("onboarding.checking")}</span>
        </div>
      ) : (
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
        <DevMockBanner />
        <Sidebar
          drawerOpen={projectDrawerOpen}
          onCloseDrawer={closeProjectDrawer}
          onToggleDrawer={toggleProjectDrawer}
          appMode={appMode}
          activeTab={activeTab}
          projectRevision={projectRevision}
          onOpenTopic={(scope, workspaceRoot, topicId) => {
            void handleOpenTopic(scope, workspaceRoot, topicId);
          }}
          onOpenWorkspace={() => {
            void openCodeWorkspace();
          }}
          onNewChat={() => {
            void startNewSession();
          }}
          onNewFreshChat={() => {
            void startFreshSession();
          }}
          onModeChange={setAppMode}
          onOpenSdd={() => {
            setAppMode("code");
            setSddOpen(true);
          }}
          onAddProject={async () => {
            await openCodeWorkspace();
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
            <>
              <Topbar
                title={topicTitle(activeTab)}
                workspacePath={showTopbarWorkspace ? topbarWorkspacePath : undefined}
                editing={topicbarEditing}
                titleDraft={topicTitleDraft}
                onTitleDraftChange={setTopicTitleDraft}
                onStartRename={startActiveTopicRename}
                onCommitRename={() => void commitActiveTopicRename()}
                onCancelRename={cancelActiveTopicRename}
                running={agentActive}
                goalLabel={goalLabel || undefined}
                sideConversationCount={sideConversationCount}
                onOpenSideConversation={() => {
                  document.querySelector(".side-conversation")?.scrollIntoView({ behavior: "smooth", block: "nearest" });
                }}
                pendingDecisionLabel={pendingDecisionLabel}
                onFocusPendingDecision={decisionPending ? focusPendingDecision : undefined}
                backgroundAttentionCount={backgroundAttentionCount}
                onFocusBackgroundSession={backgroundAttentionCount > 0 ? focusBackgroundSession : undefined}
              />
              <OpenTabsBar
                tabs={openTabs}
                activeTabId={activeTabId}
                onSelectTab={(tabId) => void switchTab(tabId)}
                onCloseTab={(tabId) => void handleCloseOpenTab(tabId)}
              />
            </>
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
              workspacePanelOpen ? "workbench__body--dock-open" : "",
            ]
              .filter(Boolean)
              .join(" ")}
            style={{ "--studio-dock-w": `${dockPanelWidth}px` } as CSSProperties}
          >
            <div className="workbench__stack">
              <div className="workbench__center">
                {showBootLoading ? (
                  <div className="loading-screen">
                    <div className="loading-screen__spinner" />
                    <span className="loading-screen__text">{bootLoadingText}</span>
                  </div>
                ) : chatMode ? (
                  <>
                    {state.meta?.ready === false && !state.meta?.startupErr ? (
                      <div className="loading-screen loading-screen--inline" role="status">
                        <div className="loading-screen__spinner loading-screen__spinner--sm" />
                        <span className="loading-screen__text">{bootLoadingText}</span>
                      </div>
                    ) : null}
                    <MessageTimeline
                    tabId={activeTabId}
                    items={deferredItems}
                    pendingUser={state.pendingUser}
                    live={deferredLive}
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
                    showConnectionRecovery={showConnectionRecovery}
                    onOpenConnectionSetup={openOnboardingManual}
                  />
                  </>
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
                    onComposerPrompt={(text) => {
                      setAppMode("code");
                      void handleSend(text);
                    }}
                    onDraftComposer={addWriteContextToComposer}
                    onPickWriteWorkspace={pickWriteWorkspace}
                    onFilesChanged={() => setProjectRevision((value) => value + 1)}
                    writeConversation={writeConversationTurns}
                    writeAgentRunning={state.running}
                    onSettingsChanged={() => void refreshMeta()}
                    onOpenHistory={() => {
                      if (appMode === "settings") {
                        void listSessions().then((sessions) => setSettingsDataModal({ kind: "history", sessions }));
                        return;
                      }
                      void openAllHistory();
                    }}
                    onOpenMemory={() => {
                      if (appMode === "settings") {
                        void fetchMemory().then((view) => setSettingsDataModal({ kind: "memory", view }));
                        return;
                      }
                      void openMemory();
                    }}
                    onOpenCapabilities={() => {
                      setAppMode("plugins");
                    }}
                    onOpenTrash={() => {
                      if (appMode === "settings") {
                        void listTrashedSessions().then((sessions) => setSettingsDataModal({ kind: "trash", sessions }));
                        return;
                      }
                      void openTrash();
                    }}
                    onConfigureProjectSandbox={() => setSandboxSetup({ reason: "manual" })}
                    onModeChange={(mode) => {
                      setAppMode(mode);
                    }}
                    onOpenDockTab={(tab) => {
                      if (appMode === "settings") {
                        setSettingsDockTab(tab);
                        return;
                      }
                      setAppMode("code");
                      openDockTab(tab, { toggle: false });
                    }}
                    onOpenTerminal={() => {
                      setAppMode("code");
                      void openNewTerminal();
                    }}
                    onOpenOnboarding={() => {
                      setOnboardingSession((session) => session + 1);
                      setOnboardingManual(true);
                    }}
                  />
                )}
              </div>

              {showWorkbenchFooter ? (
                <div className="workbench__footer" ref={footerRef}>
                  <div className={`workbench__footer-stack${terminalPanelVisible ? " workbench__footer-stack--terminal-open" : ""}`}>
                    <div className="workbench__composer-zone">
                      {showConnectionRecovery ? (
                        <ConnectionRecoveryBanner onOpenSetup={openOnboardingManual} />
                      ) : null}
                      <AgentDecisionLayer
                        approval={state.approval}
                        ask={state.ask}
                        surface={appMode === "write" ? "write" : "code"}
                        planToolCount={planToolCount}
                        onApprove={(allow, session, persist) => {
                          clearAgentDecisionNotifications();
                          if (state.approval?.tool === "exit_plan_mode" && allow) applyMode("normal");
                          if (state.approval) approve(state.approval.id, allow, session, persist);
                        }}
                        onRevisePlan={(text) => {
                          setPendingPlanRevision(text);
                          if (state.approval) approve(state.approval.id, false, false, false);
                        }}
                        onExitPlan={() => {
                          applyMode("normal");
                          if (state.approval) approve(state.approval.id, false, false, false);
                        }}
                        onAnswerAsk={handleAnswerQuestion}
                        onDismissAsk={() => {
                          const askId = state.ask?.id;
                          if (askId) handleAnswerQuestion(askId, []);
                        }}
                      />
                      {composerAgentRunning ? (
                        <TurnProgressLine
                          running={state.running}
                          turnStartAt={state.turnStartAt}
                          items={state.items}
                        />
                      ) : null}
                      <FloatingComposer
                        key={appMode === "write" ? "write" : "code"}
                        composerSurface={appMode === "write" ? "write" : "code"}
                        running={composerAgentRunning}
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
                        disabled={isComposerSendDisabled(state.meta, state.approval, state.ask, runtimeReady)}
                        decisionPending={state.approval != null || state.ask != null}
                        ready={state.meta?.ready === true}
                        turnStartAt={state.turnStartAt}
                        turnTokens={state.turnTokens}
                        retry={state.retry}
                        workspaceRefreshSignal={projectRevision}
                        showWorkspaceSwitcher
                        workspaceNone={composerWorkspaceNone}
                        onUseNoWorkspace={handleUseNoWorkspace}
                        terminalSessions={
                          chatMode ? terminalTabs.map((tab) => ({ id: tab.id, label: tab.title })) : []
                        }
                        onTerminalSessionOpen={
                          chatMode
                            ? (id) => {
                                setActiveTerminalId(id);
                                restoreTerminalPanel();
                              }
                            : undefined
                        }
                        onTerminalSessionClose={chatMode ? closeTerminalTab : undefined}
                        browserSessions={
                          chatMode && browserActive
                            ? browserTabs.map((tab) => ({ id: tab.id, label: tab.title }))
                            : []
                        }
                        onBrowserSessionOpen={
                          chatMode
                            ? (id) => {
                                setActiveBrowserTabId(id);
                                openDockTab("browser", { toggle: false });
                              }
                            : undefined
                        }
                        onBrowserSessionClose={chatMode ? closeBrowserTab : undefined}
                        pagePreviewActive={chatMode && pagePreviewActive}
                        pagePreviewLabel={pagePreviewPath ?? undefined}
                        onPageSessionOpen={
                          chatMode && pagePreviewActive
                            ? () => openDockTab("page", { toggle: false })
                            : undefined
                        }
                        onPageSessionClose={
                          chatMode && pagePreviewActive ? () => setPagePreviewPath(null) : undefined
                        }
                        context={state.context}
                        usage={state.usage}
                        balance={state.balance}
                        sessionCost={state.sessionCost}
                        sessionCurrency={state.sessionCurrency}
                      />
                    </div>
                    {terminalPanelShown && !filePreviewComposerOpen && terminalTabs.length > 0 && resolvedActiveTerminalId && (
                      <BottomTerminalPanel
                        key={terminalMotionKey}
                        height={terminalAnimHeight}
                        cwd={composerCwd}
                        tabs={terminalTabs}
                        activeId={resolvedActiveTerminalId}
                        onActiveChange={setActiveTerminalId}
                        onNewTerminal={() => void openNewTerminal()}
                        onCloseTab={closeTerminalTab}
                        onMinimizePanel={minimizeTerminalPanel}
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
                  onDoubleClick={resetFilePreviewWidthFromDock}
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

            {showRightDock && dockMounted && (
              <>
                {workspacePanelOpen ? (
                  <button
                    className="workbench__resizer workbench__resizer--dock wails-no-drag"
                    type="button"
                    role="separator"
                    aria-orientation="vertical"
                    aria-label={t("rightDock.resize")}
                    onPointerDown={startDockResize}
                    onKeyDown={resizeDockWithKeyboard}
                    onDoubleClick={resetDockWidthFromDefault}
                  />
                ) : null}
                <div
                  className={`workbench__dock-slot${dockBackgroundSessions && !workspacePanelOpen ? " workbench__dock-slot--background" : ""}`}
                  style={
                    {
                      width: workspacePanelOpen ? dockPanelWidth : 0,
                      minWidth: workspacePanelOpen ? undefined : 0,
                      flex: workspacePanelOpen ? undefined : "0 0 0px",
                    } as CSSProperties
                  }
                >
                  <RightDock
                    key={dockMotionKey}
                    open={workspacePanelOpen}
                    background={dockBackgroundSessions && !workspacePanelOpen}
                    closing={dockClosing}
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
                    browserTabs={browserTabs}
                    activeBrowserTabId={activeBrowserTabId}
                    onBrowserTabChange={setActiveBrowserTabId}
                    onCloseBrowserTab={handleCloseBrowserTab}
                    onNewBrowserTab={() => openWebPreview()}
                    onBrowserTabUrlChange={(id, url, title) => updateBrowserTab(id, { url, title })}
                    pagePreviewPath={pagePreviewPath}
                    onPagePreviewPathChange={setPagePreviewPath}
                    onPreviewPage={openPagePreview}
                  />
                </div>
              </>
            )}

            {(chatMode || appMode === "write") && (
              <StudioToolRail
                dockOpen={workspacePanelOpen}
                activeDockTab={
                  workspacePanelOpen && rightDockMode !== "browser" && rightDockMode !== "page"
                    ? rightDockMode
                    : null
                }
                onHubPress={openDockHub}
                onOpenDockTab={(tab) => openDockTab(tab, { toggle: false })}
                onOpenPreviewMode={togglePreviewMode}
              />
            )}
          </div>
        </div>
      </div>
      )}

      {memView !== null && (
        <Suspense fallback={null}>
          <LazyMemoryPanel
            view={memView}
            presentation="drawer"
            onClose={closeMemory}
            onRemember={onRemember}
            onForget={onForget}
            onSaveDoc={onSaveDoc}
          />
        </Suspense>
      )}

      {histView !== null && (
        <Suspense fallback={null}>
          <HistoryPanel
            presentation="drawer"
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
        </Suspense>
      )}

      {settingsDockTab !== null ? (
        <SettingsDockModal
          tab={settingsDockTab}
          cwd={state.meta?.cwd}
          refreshKey={projectRevision}
          onClose={() => setSettingsDockTab(null)}
        />
      ) : null}

      {settingsDataModal !== null ? (
        <SettingsWorkspaceDataModal
          state={settingsDataModal}
          running={state.running}
          onClose={() => setSettingsDataModal(null)}
          onResume={onResumeSession}
          onPreview={previewSession}
          onDelete={onDeleteSession}
          onRename={onRenameSession}
          onRestore={onRestoreTrashedSession}
          onPurge={onPurgeTrashedSession}
          onPurgeAll={onPurgeAllTrashedSessions}
          onRemember={onRemember}
          onForget={onForget}
          onSaveDoc={onSaveDoc}
        />
      ) : null}

      {(onboardingManual || onboardingGate === true) && (
        <OnboardingOverlay
          key={onboardingSession}
          manual={onboardingManual}
          onComplete={() => {
            setOnboardingManual(false);
            setOnboardingGate(false);
          }}
        />
      )}
      {sandboxSetup && (
        <SandboxSetupOverlay
          reason={sandboxSetup.reason}
          onCancel={() => setSandboxSetup(null)}
          onComplete={() => setSandboxSetup(null)}
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
        busy={sideChatBusy}
        onSend={(text) => void handleSideSend(text)}
        onClose={() => {
          setSideMessages([]);
          setSideConversationCount(0);
        }}
      />
      </div>
    </ShellExpandProvider>
  );
}
