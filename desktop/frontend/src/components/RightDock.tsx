import { PanelRightClose } from "lucide-react";

import { useT } from "../lib/i18n";

import { dockHubForTab } from "../lib/dockHubs";

import { dockHubLabel, dockTabLabel, dockTabsForHub } from "./DockHubButtons";

import { BrowserPanel } from "./BrowserPanel";

import { ChangesPanel } from "./ChangesPanel";

import { ContextPanel } from "./ContextPanel";

import { FilesPanel } from "./FilesPanel";

import { GitPanel } from "./GitPanel";

import { TodoPanel } from "./TodoPanel";

import type { BalanceInfo, ContextInfo, EffortInfo, Mode, WireUsage } from "../lib/types";

import type { Todo } from "../lib/tools";

import type { RightDockTab } from "./Topbar";

export interface RightDockProps {

  open: boolean;

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

}



export function RightDock({

  open,

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

}: RightDockProps) {

  const t = useT();

  if (!open) return null;



  const hub = dockHubForTab(tab);

  const hubTabs = dockTabsForHub(hub);

  const headTitle = `${dockHubLabel(hub, t)} · ${dockTabLabel(tab, t)}`;



  return (

    <aside className="right-dock" aria-label={t("rightDock.workbench")}>

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

            onOpenChangesTab={() => onTabChange("changes")}

          />

        )}

        {tab === "changes" && (
          <ChangesPanel
            cwd={cwd}
            refreshKey={refreshKey}
            activeFilePath={filePreviewPath}
            onOpenFile={(path) => onOpenFile?.(path, "changes")}
            onAddToChat={onAddToChat}
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
            onAddToChat={onAddToChat}
          />
        )}

        {tab === "todo" && (
          <TodoPanel
            todos={todos ?? []}
            stale={todoStale}
            onDismiss={onDismissTodos ?? (() => {})}
            onStartPlan={onStartPlan}
          />
        )}

        {tab === "browser" && <BrowserPanel />}

      </div>

    </aside>

  );

}

