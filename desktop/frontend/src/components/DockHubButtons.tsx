import { useRef, useState } from "react";
import type { MutableRefObject, ReactNode } from "react";
import {
  CheckSquare,
  ChevronDown,
  CircleGauge,
  FileText,
  GitBranch,
  GitCommitHorizontal,
  ListChecks,
  Monitor,
  PanelRight,
  SquareTerminal,
} from "lucide-react";
import { AnchoredPopover } from "./AnchoredPopover";
import type { RightDockTab } from "./Topbar";
import { useT, type Translator } from "../lib/i18n";
import type { DictKey } from "../locales/en";
import {
  DOCK_HUBS,
  dockHubDef,
  dockHubForTab,
  getPreviewPanelState,
  previewModesForHub,
  type DockHub,
  type PreviewMode,
} from "../lib/dockHubs";

const TAB_META: Record<RightDockTab, { icon: ReactNode; labelKey: string; fallback: string }> = {
  context: { icon: <CircleGauge size={14} />, labelKey: "rightDock.overview", fallback: "Overview" },
  changes: { icon: <GitBranch size={14} />, labelKey: "rightDock.tab.changes", fallback: "Changes" },
  todo: { icon: <CheckSquare size={14} />, labelKey: "rightDock.tab.todo", fallback: "To-dos" },
  git: { icon: <GitCommitHorizontal size={14} />, labelKey: "rightDock.tab.git", fallback: "Git" },
  browser: { icon: <Monitor size={14} />, labelKey: "browser.title", fallback: "Browser" },
  files: { icon: <FileText size={14} />, labelKey: "rightDock.tab.files", fallback: "Files" },
};

const PREVIEW_META: Record<PreviewMode, { icon: ReactNode; labelKey: string; fallback: string }> = {
  browser: { icon: <Monitor size={14} />, labelKey: "browser.title", fallback: "Browser" },
  terminal: { icon: <SquareTerminal size={14} />, labelKey: "terminal.title", fallback: "Terminal" },
};

const HUB_META: Record<DockHub, { icon: ReactNode; labelKey: "dockHub.context" | "dockHub.work" | "dockHub.preview" }> = {
  context: { icon: <CircleGauge size={13} />, labelKey: "dockHub.context" },
  work: { icon: <ListChecks size={13} />, labelKey: "dockHub.work" },
  preview: { icon: <PanelRight size={13} />, labelKey: "dockHub.preview" },
};

export interface DockHubButtonsProps {
  dockOpen: boolean;
  activeDockTab?: RightDockTab | null;
  terminalOpen?: boolean;
  onHubPress: (hub: DockHub) => void;
  onOpenDockTab: (tab: RightDockTab) => void;
  onOpenPreviewMode: (mode: PreviewMode) => void;
}

export function DockHubButtons({
  dockOpen,
  activeDockTab,
  terminalOpen = false,
  onHubPress,
  onOpenDockTab,
  onOpenPreviewMode,
}: DockHubButtonsProps) {
  const t = useT();
  const [menuHub, setMenuHub] = useState<DockHub | null>(null);
  const menuAnchorRef = useRef<HTMLButtonElement | null>(null) as MutableRefObject<HTMLButtonElement | null>;

  const activeHub = activeDockTab ? dockHubForTab(activeDockTab) : null;
  const previewState = getPreviewPanelState(terminalOpen, dockOpen, activeDockTab);

  const openMenu = (hub: DockHub, anchor: HTMLButtonElement) => {
    menuAnchorRef.current = anchor;
    setMenuHub((current) => (current === hub ? null : hub));
  };

  const pickTab = (tab: RightDockTab) => {
    setMenuHub(null);
    onOpenDockTab(tab);
  };

  const pickPreviewMode = (mode: PreviewMode) => {
    setMenuHub(null);
    onOpenPreviewMode(mode);
  };

  const hubIsActive = (hub: DockHub): boolean => {
    if (hub === "preview") return previewState.terminal || previewState.browser;
    return dockOpen && activeHub === hub;
  };

  const previewModeChecked = (mode: PreviewMode): boolean => {
    return mode === "terminal" ? previewState.terminal : previewState.browser;
  };

  const menuItemCount = (hub: DockHub): number => {
    if (hub === "preview") return previewModesForHub(hub).length;
    return dockHubDef(hub).tabs.length;
  };

  return (
    <div className="topbar__dock-hubs" role="toolbar" aria-label={t("rightDock.views")}>
      {DOCK_HUBS.map((hub) => {
        const meta = HUB_META[hub.id];
        const hubActive = hubIsActive(hub.id);
        const hasMenu = menuItemCount(hub.id) > 1;
        return (
          <div key={hub.id} className={`topbar__hub${hubActive ? " topbar__hub--active" : ""}`}>
            <div className="topbar__hub-controls">
              <button
                type="button"
                className="topbar__hub-main"
                onClick={() => onHubPress(hub.id)}
                aria-label={t(meta.labelKey)}
                aria-pressed={hubActive}
              >
                <span className="topbar__hub-icon" aria-hidden="true">
                  {meta.icon}
                </span>
              </button>
              {hasMenu && (
                <>
                  <span className="topbar__hub-divider" aria-hidden="true" />
                  <button
                    type="button"
                    className={`topbar__hub-menu${menuHub === hub.id ? " topbar__hub-menu--open" : ""}`}
                    onClick={(event) => openMenu(hub.id, event.currentTarget)}
                    aria-label={t("dockHub.openMenu", { hub: t(meta.labelKey) })}
                    aria-expanded={menuHub === hub.id}
                    aria-haspopup="menu"
                  >
                    <ChevronDown size={11} />
                  </button>
                </>
              )}
            </div>
            <span className="topbar__hub-label">{t(meta.labelKey)}</span>
          </div>
        );
      })}

      <AnchoredPopover
        open={menuHub !== null}
        anchorRef={menuAnchorRef}
        onClose={() => setMenuHub(null)}
        className="dock-hub-menu"
        align="end"
        placement="bottom"
        offset={6}
      >
        {menuHub === "preview" && (
          <div className="dock-hub-menu__list" role="menu">
            {previewModesForHub("preview").map((mode) => {
              const modeMeta = PREVIEW_META[mode];
              const checked = previewModeChecked(mode);
              return (
                <button
                  key={mode}
                  type="button"
                  role="menuitemcheckbox"
                  aria-checked={checked}
                  className={`dock-hub-menu__item${checked ? " dock-hub-menu__item--active" : ""}`}
                  onClick={() => pickPreviewMode(mode)}
                >
                  <span className="dock-hub-menu__item-main">
                    {modeMeta.icon}
                    <span>{t(modeMeta.labelKey as DictKey) || modeMeta.fallback}</span>
                  </span>
                  {checked && <span className="dock-hub-menu__check">✓</span>}
                </button>
              );
            })}
          </div>
        )}
        {menuHub && menuHub !== "preview" && (
          <div className="dock-hub-menu__list" role="menu">
            {dockHubDef(menuHub).tabs.map((tab) => {
              const tabMeta = TAB_META[tab];
              const selected = dockOpen && activeDockTab === tab;
              return (
                <button
                  key={tab}
                  type="button"
                  role="menuitemradio"
                  aria-checked={selected}
                  className={`dock-hub-menu__item${selected ? " dock-hub-menu__item--active" : ""}`}
                  onClick={() => pickTab(tab)}
                >
                  <span className="dock-hub-menu__item-main">
                    {tabMeta.icon}
                    <span>{t(tabMeta.labelKey as DictKey) || tabMeta.fallback}</span>
                  </span>
                  {selected && <span className="dock-hub-menu__check">✓</span>}
                </button>
              );
            })}
          </div>
        )}
      </AnchoredPopover>
    </div>
  );
}

export function dockTabLabel(tab: RightDockTab, t: Translator): string {
  const meta = TAB_META[tab];
  return t(meta.labelKey as DictKey) || meta.fallback;
}

export function dockHubLabel(hub: DockHub, t: Translator): string {
  return t(HUB_META[hub].labelKey);
}

export function dockTabsForHub(hub: DockHub): RightDockTab[] {
  return dockHubDef(hub).tabs;
}
