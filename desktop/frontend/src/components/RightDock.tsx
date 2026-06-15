import { PanelRightClose } from "lucide-react";

import { useT } from "../lib/i18n";

import { dockHubForTab } from "../lib/dockHubs";

import { dockHubLabel, dockTabLabel, dockTabsForHub } from "./DockHubButtons";

import type { BrowserTab } from "../lib/useBrowserPanel";

import { ChangesPanel } from "./ChangesPanel";

import { ContextPanel } from "./ContextPanel";

import { FilesPanel } from "./FilesPanel";

import { GitPanel } from "./GitPanel";

import { TodoPanel } from "./TodoPanel";

import type { BalanceInfo, ContextInfo, EffortInfo, Mode, WireUsage } from "../lib/types";

import type { Todo } from "../lib/tools";

import type { RightDockTab } from "./Topbar";

import type { CodeReviewState } from "./CodeReviewSection";

import type { ReviewMode, ReviewScope } from "../lib/codeReview";

export interface RightDockProps {

  open: boolean;

  background?: boolean;

  closing?: boolean;

  tab: RightDockTab;

  onTabChange: (tab: RightDockTab) => void;

  onClose: () => void;

  tabId?: string;

  context: ContextInfo;

  usage?: WireUsage;

  sessionCost?: number;

  sessionCurrency?: string;

  scopeLabel?: string;

  refreshKey?: number;

  modelLabel?: string;

  mode?: Mode;

  effort?: EffortInfo;

  balance?: BalanceInfo;

  running?: boolean;

  cwd?: string;

  onAddToChat?: (text: string) => void;

  filePreviewPath?: string | null;

  onOpenFile?: (path: string, dockTab?: RightDockTab) => void;

  todos?: Todo[];

  todoStale?: boolean;

  onDismissTodos?: () => void;

  onStartPlan?: () => void;

  onSyncTodoProgress?: () => void;

  todoSyncing?: boolean;

  codeReview?: CodeReviewState;

  onRunCodeReview?: (mode: ReviewMode, scope: ReviewScope, paths: string[]) => void;

  onClearCodeReview?: () => void;

  browserExpanded?: boolean;

  onToggleBrowserExpanded?: () => void;

  browserTabs?: BrowserTab[];

  activeBrowserTabId?: string | null;

  onBrowserTabChange?: (id: string) => void;

  onCloseBrowserTab?: (id: string) => void;

  onNewBrowserTab?: () => void;

  onBrowserTabUrlChange?: (id: string, url: string, title?: string) => void;

  pagePreviewPath?: string | null;

  onPagePreviewPathChange?: (path: string) => void;

  onPreviewPage?: (path: string) => void;

}



export function RightDock({

  open,

  background = false,

  closing = false,

  tab,

  onTabChange,

  onClose,

  tabId,

  context,

  usage,

  sessionCost,

  sessionCurrency,

  scopeLabel,

  refreshKey,

  modelLabel,

  mode,

  effort,

  balance,

  running,

  cwd,

  onAddToChat,

  filePreviewPath,

  onOpenFile,

  todos,

  todoStale,

  onDismissTodos,

  onStartPlan,

  onSyncTodoProgress,

  todoSyncing,

  codeReview,

  onRunCodeReview,

  onClearCodeReview,

  onPreviewPage,

}: RightDockProps) {

  const t = useT();

  if (!open && !background) return null;



  const hub = dockHubForTab(tab);

  const hubTabs = hub === "preview" ? [] : dockTabsForHub(hub);

  const headTitle = hub === "preview" && tab === "browser"
    ? dockTabLabel(tab, t)
    : `${dockHubLabel(hub, t)} · ${dockTabLabel(tab, t)}`;



  return (

    <aside
      className={[
        "right-dock",
        background ? "right-dock--background" : "",
        closing ? "motion-panel--closing" : "",
      ]
        .filter(Boolean)
        .join(" ")}
      aria-label={t("rightDock.workbench")}
    >

      {open ? (
        <div className="right-dock__head">

        <div className="right-dock__head-main">

          <div className="right-dock__head-title">{headTitle}</div>

          {hubTabs.length > 1 && (

            <div className="right-dock__subtabs" role="tablist" aria-label={t("dockHub.subTabs")}>

              {hubTabs.map((id) => (

                <button

                  key={id}

                  type="button"

                  role="tab"

                  aria-selected={tab === id}

                  className={`right-dock__subtab${tab === id ? " right-dock__subtab--active" : ""}`}

                  onClick={() => onTabChange(id)}

                >

                  {dockTabLabel(id, t)}

                </button>

              ))}

            </div>

          )}

        </div>

        <button type="button" className="right-dock__close" onClick={onClose} aria-label={t("rightDock.collapse")}>

          <PanelRightClose size={16} />

        </button>

      </div>
      ) : null}

      <div className="right-dock__body">

        {tab === "context" && (

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

            onOpenChangesTab={() => onTabChange("changes")}

            onOpenGitTab={() => onTabChange("git")}

          />

        )}

        {tab === "changes" && (
          <ChangesPanel
            cwd={cwd}
            refreshKey={refreshKey}
            activeFilePath={filePreviewPath}
            running={running}
            review={codeReview}
            onOpenFile={(path) => onOpenFile?.(path, "changes")}
            onAddToChat={onAddToChat}
            onRunReview={onRunCodeReview}
            onClearReview={onClearCodeReview}
          />
        )}

        {tab === "git" && (
          <GitPanel
            cwd={cwd}
            refreshKey={refreshKey}
            activeFilePath={filePreviewPath}
            onOpenFile={(path) => onOpenFile?.(path, "git")}
            onAddToChat={onAddToChat}
          />
        )}

        {tab === "files" && (
          <FilesPanel
            cwd={cwd}
            refreshKey={refreshKey}
            activeFilePath={filePreviewPath}
            onOpenFile={(path) => onOpenFile?.(path, "files")}
            onPreviewPage={onPreviewPage}
            onAddToChat={onAddToChat}
          />
        )}

        {tab === "todo" && (
          <TodoPanel
            todos={todos ?? []}
            stale={todoStale}
            onDismiss={onDismissTodos ?? (() => {})}
            onStartPlan={onStartPlan}
            onSyncProgress={onSyncTodoProgress}
            syncing={todoSyncing}
          />
        )}

      </div>

    </aside>

  );

}

