import { useRef, useState } from "react";

import type { MutableRefObject } from "react";

import {

  CheckSquare,

  CircleGauge,

  FileText,

  GitBranch,

  GitCommitHorizontal,

  Monitor,

  PanelRightClose,

  Plus,

  SquareTerminal,

} from "lucide-react";

import { AnchoredPopover } from "../AnchoredPopover";

import { useT } from "../../lib/i18n";

import type { DictKey } from "../../locales/en";

import type { SidebarPrimaryTab, SidebarProfile } from "../../lib/sidebarViews";
import { sidebarAddActionsForProfile, sidebarPanelTabsForProfile } from "../../lib/sidebarViews";

import type { SidebarSessionKind, SidebarSessionTab } from "../../lib/sidebarSessionTabs";

import { SidebarSessionTabs } from "./SidebarSessionTabs";

import { Tooltip } from "../Tooltip";



export interface SidebarTabBarProps {

  active: SidebarPrimaryTab | null;

  onSelect: (tab: SidebarPrimaryTab) => void;

  onNewTerminal: () => void;

  onNewBrowser: () => void;

  sessions: SidebarSessionTab[];

  activeSessionKind: SidebarSessionKind | null;

  activeSessionId: string | null;

  onSelectSession: (kind: SidebarSessionKind, id: string) => void;

  onCloseSession: (kind: SidebarSessionKind, id: string) => void;

  onClose?: () => void;

  profile?: SidebarProfile;

}



const PANEL_TAB_META: Record<
  SidebarPrimaryTab,
  { icon: typeof GitBranch; labelKey: string; fallback: string }
> = {
  changes: { icon: GitBranch, labelKey: "rightDock.tab.changes", fallback: "Changes" },
  files: { icon: FileText, labelKey: "rightDock.tab.files", fallback: "Files" },
  git: { icon: GitCommitHorizontal, labelKey: "rightDock.tab.git", fallback: "Git" },
  context: { icon: CircleGauge, labelKey: "rightDock.overview", fallback: "Overview" },
  todo: { icon: CheckSquare, labelKey: "rightDock.tab.todo", fallback: "To-dos" },
  browser: { icon: Monitor, labelKey: "sidebar.newBrowser", fallback: "Browser" },
  terminal: { icon: SquareTerminal, labelKey: "sidebar.newTerminal", fallback: "Terminal" },
};

const ADD_ACTION_META: Record<
  "terminal" | "browser",
  { icon: typeof SquareTerminal; labelKey: string; fallback: string }
> = {
  terminal: { icon: SquareTerminal, labelKey: "sidebar.newTerminal", fallback: "Terminal" },
  browser: { icon: Monitor, labelKey: "sidebar.newBrowser", fallback: "Browser" },
};



export function SidebarTabBar({

  active,

  onSelect,

  onNewTerminal,

  onNewBrowser,

  sessions,

  activeSessionKind,

  activeSessionId,

  onSelectSession,

  onCloseSession,

  onClose,
  profile = "code",
}: SidebarTabBarProps) {
  const t = useT();
  const panelTabs = sidebarPanelTabsForProfile(profile);
  const addActions = sidebarAddActionsForProfile(profile);

  const addAnchorRef = useRef<HTMLButtonElement | null>(null) as MutableRefObject<HTMLButtonElement | null>;

  const [addOpen, setAddOpen] = useState(false);



  return (

    <header className="unified-sidebar__head wails-no-drag">

      <div className="unified-sidebar__head-row">

        <nav className="unified-sidebar__panel-tabs" role="tablist" aria-label={t("rightDock.views")}>

          {panelTabs.map((id) => {
            const { icon: Icon, labelKey, fallback } = PANEL_TAB_META[id];
            return (
            <Tooltip key={id} label={t(labelKey as DictKey) || fallback}>

              <button

                type="button"

                role="tab"

                aria-selected={active === id}

                aria-label={t(labelKey as DictKey) || fallback}

                className={`unified-sidebar__tab${active === id ? " unified-sidebar__tab--active" : ""}`}

                onClick={() => onSelect(id)}

              >

                <Icon size={15} strokeWidth={1.75} />

              </button>

            </Tooltip>
            );
          })}

          <Tooltip label={t("sidebar.newTab")}>

            <button

              ref={addAnchorRef}

              type="button"

              className={`unified-sidebar__tab unified-sidebar__tab--add${addOpen ? " unified-sidebar__tab--active" : ""}`}

              aria-label={t("sidebar.newTab")}

              aria-expanded={addOpen}

              aria-haspopup="menu"

              onClick={() => setAddOpen((open) => !open)}

            >

              <Plus size={15} strokeWidth={1.75} />

            </button>

          </Tooltip>

        </nav>



        <AnchoredPopover

          open={addOpen}

          anchorRef={addAnchorRef}

          onClose={() => setAddOpen(false)}

          className="dock-hub-menu unified-sidebar__add-menu"

          align="end"

          placement="bottom"

          offset={6}

        >

          <div className="dock-hub-menu__list" role="menu">
            {addActions.map((action) => {
              const { icon: Icon, labelKey, fallback } = ADD_ACTION_META[action];
              return (
              <button
                key={action}
                type="button"
                role="menuitem"
                className="dock-hub-menu__item"
                onClick={() => {
                  setAddOpen(false);
                  if (action === "terminal") onNewTerminal();
                  else onNewBrowser();
                }}
              >
                <span className="dock-hub-menu__item-main">
                  <Icon size={14} />
                  <span>{t(labelKey as DictKey) || fallback}</span>
                </span>
              </button>
              );
            })}
          </div>

        </AnchoredPopover>

        {onClose ? (
          <Tooltip label={t("rightDock.collapse")}>
            <button
              type="button"
              className="unified-sidebar__close"
              onClick={onClose}
              aria-label={t("rightDock.collapse")}
            >
              <PanelRightClose size={16} strokeWidth={1.75} />
            </button>
          </Tooltip>
        ) : null}

      </div>



      <SidebarSessionTabs

        tabs={sessions}

        activeKind={activeSessionKind}

        activeId={activeSessionId}

        onSelect={onSelectSession}

        onClose={onCloseSession}

      />

    </header>

  );

}

