import { useCallback, useMemo, useRef, useState } from "react";
import type { PreviewMode } from "../lib/dockHubs";
import { sidebarShowsBranchBar, type SidebarPrimaryTab, type SidebarProfile, defaultSidebarPanelTab } from "../lib/sidebarViews";
import { buildSidebarSessionTabs, type SidebarSessionKind } from "../lib/sidebarSessionTabs";
import type { RightDockTab } from "./Topbar";
import type { RightDockProps } from "./rightDockTypes";
import { SidebarTabBar } from "./sidebar/SidebarTabBar";
import { SidebarBranchBar } from "./sidebar/SidebarBranchBar";
import { SidebarChangesView } from "./sidebar/SidebarChangesView";
import { ContextPanel } from "./ContextPanel";
import { FilesPanel } from "./FilesPanel";
import { GitPanel } from "./GitPanel";
import { TodoPanel } from "./TodoPanel";
import { BrowserPanel } from "./BrowserPanel";
import { FilePreviewPanel } from "./FilePreviewPanel";
import { PagePreviewPanel } from "./PagePreviewPanel";
import { PreviewTerminalPane } from "./PreviewTerminalPane";
import type { BrowserTab } from "../lib/useBrowserPanel";
import type { TerminalTab } from "./TerminalPanel";
import type { ToolFileDiff } from "../lib/tools";
import { useWorkspaceChanges } from "../lib/useWorkspaceChanges";

export interface UnifiedRightSidebarProps extends Omit<RightDockProps, "onTabChange"> {
  previewColumnOpen: boolean;
  previewMode: PreviewMode;
  sidebarExpanded: boolean;
  onSelectSidebarTab: (tab: SidebarPrimaryTab) => void;
  onSelectSidebarSession: (kind: SidebarSessionKind, id: string) => void;
  sidebarBodyTab: SidebarPrimaryTab;
  onToggleSidebarExpanded: () => void;
  onAddBrowser: () => void;
  fileDiff?: ToolFileDiff | null;
  onCloseFile?: () => void;
  pagePath?: string | null;
  onPagePathChange?: (path: string) => void;
  workspaceRoot?: string;
  browserTabs?: BrowserTab[];
  activeBrowserTabId?: string | null;
  onBrowserTabChange?: (id: string) => void;
  onCloseBrowserTab?: (id: string) => void;
  onNewBrowserTab?: () => void;
  onBrowserTabUrlChange?: (id: string, url: string, title?: string) => void;
  terminalTabs?: TerminalTab[];
  activeTerminalId?: string | null;
  onTerminalTabChange?: (id: string) => void;
  onNewTerminal?: () => void;
  onCloseTerminalTab?: (id: string, index: number) => void;
  sidebarProfile?: SidebarProfile;
}

export function UnifiedRightSidebar({
  open,
  background = false,
  closing = false,
  tab: _tab,
  onClose,
  previewColumnOpen,
  previewMode,
  sidebarExpanded,
  onSelectSidebarTab,
  onSelectSidebarSession,
  sidebarBodyTab,
  onToggleSidebarExpanded: _onToggleSidebarExpanded,
  onAddBrowser,
  cwd,
  refreshKey,
  filePreviewPath,
  fileDiff,
  onCloseFile,
  onOpenFile,
  onAddToChat,
  onPreviewPage,
  pagePath,
  onPagePathChange,
  workspaceRoot,
  browserTabs,
  activeBrowserTabId,
  onBrowserTabChange,
  onCloseBrowserTab,
  onNewBrowserTab,
  onBrowserTabUrlChange,
  terminalTabs,
  activeTerminalId,
  onTerminalTabChange,
  onNewTerminal,
  onCloseTerminalTab,
  tabId,
  context,
  usage,
  sessionCost,
  sessionCurrency,
  scopeLabel,
  modelLabel,
  mode,
  effort,
  balance,
  running,
  todos,
  todoStale,
  onDismissTodos,
  onStartPlan,
  onSyncTodoProgress,
  todoSyncing,
  sidebarProfile = "code",
}: UnifiedRightSidebarProps) {
  const primaryTab = sidebarBodyTab;
  const sessionTabs = useMemo(
    () => buildSidebarSessionTabs(browserTabs ?? [], terminalTabs ?? []),
    [browserTabs, terminalTabs],
  );
  const panelActive: SidebarPrimaryTab | null =
    primaryTab === "changes" ||
    primaryTab === "files" ||
    primaryTab === "git" ||
    primaryTab === "context" ||
    primaryTab === "todo"
      ? primaryTab
      : null;
  const activeSessionKind: SidebarSessionKind | null =
    primaryTab === "browser" || primaryTab === "terminal" ? primaryTab : null;
  const activeSessionId =
    activeSessionKind === "browser"
      ? activeBrowserTabId ?? null
      : activeSessionKind === "terminal"
        ? activeTerminalId ?? null
        : null;

  const handleCloseSession = useCallback(
    (kind: SidebarSessionKind, id: string) => {
      const closingActive = activeSessionKind === kind && activeSessionId === id;
      if (kind === "browser") {
        onCloseBrowserTab?.(id);
      } else {
        const index = (terminalTabs ?? []).findIndex((tab) => tab.id === id);
        if (index >= 0) onCloseTerminalTab?.(id, index);
      }
      if (!closingActive) return;
      const remainingBrowsers = (browserTabs ?? []).filter((tab) => tab.id !== id);
      const remainingTerminals = (terminalTabs ?? []).filter((tab) => tab.id !== id);
      if (kind === "browser" && remainingBrowsers.length > 0) {
        onSelectSidebarSession("browser", remainingBrowsers[remainingBrowsers.length - 1]!.id);
        return;
      }
      if (remainingTerminals.length > 0) {
        onSelectSidebarSession("terminal", remainingTerminals[remainingTerminals.length - 1]!.id);
        return;
      }
      if (remainingBrowsers.length > 0) {
        onSelectSidebarSession("browser", remainingBrowsers[remainingBrowsers.length - 1]!.id);
        return;
      }
      onSelectSidebarTab(defaultSidebarPanelTab(sidebarProfile));
    },
    [
      activeBrowserTabId,
      activeSessionId,
      activeSessionKind,
      browserTabs,
      sidebarProfile,
      onCloseBrowserTab,
      onCloseTerminalTab,
      onSelectSidebarSession,
      onSelectSidebarTab,
      terminalTabs,
    ],
  );
  const { changes, loadChanges } = useWorkspaceChanges(cwd, refreshKey);
  const gitAvailable = changes?.gitAvailable ?? false;
  const [changeStats, setChangeStats] = useState({ count: 0, added: 0, removed: 0 });
  const [excluded, setExcluded] = useState<Set<string>>(new Set());
  const stageBeforeCommitRef = useRef<(() => Promise<void>) | null>(null);

  const showBranchBar = sidebarShowsBranchBar(primaryTab);
  const showInlineFilePreview =
    previewColumnOpen && (previewMode === "file" || previewMode === "page") && primaryTab === "files";
  const showTerminalPane = primaryTab === "terminal";
  const showBrowserPane = primaryTab === "browser";
  const terminalTabCount = terminalTabs?.length ?? 0;
  const browserTabCount = browserTabs?.length ?? 0;
  const resolvedTerminalId = activeTerminalId ?? "";

  const handleOpenFile = useCallback(
    (path: string, dockTab: RightDockTab = "files") => {
      onOpenFile?.(path, dockTab);
    },
    [onOpenFile],
  );

  if (!open && !background) return null;

  return (
    <aside
      className={[
        "unified-sidebar",
        background ? "unified-sidebar--background" : "",
        closing ? "motion-panel--closing" : "",
        sidebarExpanded ? "unified-sidebar--expanded" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      aria-label="Sidebar"
    >
      <SidebarTabBar
        active={panelActive}
        onSelect={onSelectSidebarTab}
        onNewTerminal={() => onNewTerminal?.()}
        onNewBrowser={onAddBrowser}
        sessions={sessionTabs}
        activeSessionKind={activeSessionKind}
        activeSessionId={activeSessionId}
        onSelectSession={onSelectSidebarSession}
        onCloseSession={handleCloseSession}
        onClose={onClose}
        profile={sidebarProfile}
      />

      {showBranchBar ? (
        <SidebarBranchBar
          cwd={cwd}
          gitAvailable={gitAvailable}
          refreshKey={refreshKey}
          changeCount={changeStats.count}
          addedLines={changeStats.added}
          removedLines={changeStats.removed}
          onRefreshChanges={() => void loadChanges()}
          onStageBeforeCommit={async () => {
            await stageBeforeCommitRef.current?.();
          }}
        />
      ) : null}

      <div className="unified-sidebar__body">
        {primaryTab === "changes" ? (
          <SidebarChangesView
            cwd={cwd}
            refreshKey={refreshKey}
            activeFilePath={filePreviewPath}
            excluded={excluded}
            onExcludedChange={setExcluded}
            onOpenFile={(path) => handleOpenFile(path, "changes")}
            onStatsChange={setChangeStats}
            onRegisterStage={(fn) => {
              stageBeforeCommitRef.current = fn;
            }}
          />
        ) : null}

        {primaryTab === "git" ? (
          <GitPanel
            cwd={cwd}
            refreshKey={refreshKey}
            activeFilePath={filePreviewPath}
            onOpenFile={(path) => handleOpenFile(path, "git")}
            onAddToChat={onAddToChat}
          />
        ) : null}

        {primaryTab === "files" && !showInlineFilePreview ? (
          <FilesPanel
            cwd={cwd}
            refreshKey={refreshKey}
            activeFilePath={filePreviewPath}
            onOpenFile={(path) => handleOpenFile(path, "files")}
            onPreviewPage={onPreviewPage}
            onAddToChat={onAddToChat}
          />
        ) : null}

        {primaryTab === "context" ? (
          <ContextPanel
            tabId={tabId}
            context={context}
            usage={usage}
            sessionCost={sessionCost}
            sessionCurrency={sessionCurrency}
            scopeLabel={scopeLabel}
            refreshKey={refreshKey}
            modelLabel={modelLabel}
            mode={mode}
            effort={effort}
            balance={balance}
            running={running}
            cwd={cwd}
            onOpenChangesTab={() => onSelectSidebarTab("changes")}
            onOpenGitTab={() => onSelectSidebarTab("git")}
          />
        ) : null}

        {primaryTab === "todo" ? (
          <TodoPanel
            todos={todos ?? []}
            stale={todoStale}
            onDismiss={onDismissTodos ?? (() => {})}
            onStartPlan={onStartPlan}
            onSyncProgress={onSyncTodoProgress}
            syncing={todoSyncing}
          />
        ) : null}

        {browserTabCount > 0 ? (
          <div className="preview-browser-pane-host" hidden={!showBrowserPane}>
            <BrowserPanel
              tabs={browserTabs ?? []}
              activeId={activeBrowserTabId ?? null}
              onActiveChange={onBrowserTabChange ?? (() => {})}
              onCloseTab={onCloseBrowserTab ?? (() => {})}
              onNewTab={onNewBrowserTab ?? (() => {})}
              onTabUrlChange={onBrowserTabUrlChange ?? (() => {})}
              embedded
              hideTabBar
            />
          </div>
        ) : showBrowserPane ? (
          <BrowserPanel
            tabs={[]}
            activeId={null}
            onActiveChange={() => {}}
            onCloseTab={() => {}}
            onNewTab={onNewBrowserTab ?? onAddBrowser}
            onTabUrlChange={() => {}}
            embedded
            hideTabBar
          />
        ) : null}

        {terminalTabCount > 0 ? (
          <div className="preview-terminal-pane-host" hidden={!showTerminalPane}>
            <PreviewTerminalPane
              tabs={terminalTabs ?? []}
              activeId={resolvedTerminalId}
              onActiveChange={onTerminalTabChange ?? (() => {})}
              onNewTerminal={onNewTerminal ?? (() => {})}
              onCloseTab={onCloseTerminalTab ?? (() => {})}
              visible={showTerminalPane}
              hideTabBar
            />
          </div>
        ) : showTerminalPane ? (
          <PreviewTerminalPane
            tabs={[]}
            activeId=""
            onActiveChange={() => {}}
            onNewTerminal={onNewTerminal ?? (() => {})}
            onCloseTab={() => {}}
            visible
            hideTabBar
          />
        ) : null}

        {showInlineFilePreview && previewMode === "file" && filePreviewPath ? (
          <FilePreviewPanel
            path={filePreviewPath}
            diff={fileDiff ?? null}
            onClose={onCloseFile ?? (() => {})}
            onAddToChat={onAddToChat}
            embedded
          />
        ) : null}

        {showInlineFilePreview && previewMode === "page" ? (
          <PagePreviewPanel
            pagePath={pagePath ?? null}
            onPagePathChange={onPagePathChange ?? (() => {})}
            refreshKey={refreshKey ?? 0}
            workspaceRoot={workspaceRoot}
            embedded
          />
        ) : null}
      </div>
    </aside>
  );
}
