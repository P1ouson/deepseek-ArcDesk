import { Monitor, SquareTerminal, X } from "lucide-react";
import { useT } from "../../lib/i18n";
import type { SidebarSessionKind, SidebarSessionTab } from "../../lib/sidebarSessionTabs";

export interface SidebarSessionTabsProps {
  tabs: SidebarSessionTab[];
  activeKind: SidebarSessionKind | null;
  activeId: string | null;
  onSelect: (kind: SidebarSessionKind, id: string) => void;
  onClose: (kind: SidebarSessionKind, id: string) => void;
}

export function SidebarSessionTabs({ tabs, activeKind, activeId, onSelect, onClose }: SidebarSessionTabsProps) {
  const t = useT();
  if (tabs.length === 0) return null;

  return (
    <div className="unified-sidebar__session-tabs" role="tablist" aria-label={t("sidebar.sessionTabs")}>
      {tabs.map((tab) => {
        const active = activeKind === tab.kind && activeId === tab.id;
        const Icon = tab.kind === "browser" ? Monitor : SquareTerminal;
        const closeLabel =
          tab.kind === "browser"
            ? t("browser.closeTab", { title: tab.title })
            : t("terminal.closeTab", { title: tab.title });
        return (
          <div
            key={`${tab.kind}-${tab.clientKey}`}
            className={`unified-sidebar__session-tab${active ? " unified-sidebar__session-tab--active" : ""}`}
            role="presentation"
          >
            <button
              type="button"
              role="tab"
              aria-selected={active}
              className="unified-sidebar__session-tab-main"
              onClick={() => onSelect(tab.kind, tab.id)}
            >
              <Icon size={12} strokeWidth={1.75} />
              <span>{tab.title}</span>
            </button>
            <button
              type="button"
              className="unified-sidebar__session-tab-close"
              aria-label={closeLabel}
              onClick={(event) => {
                event.stopPropagation();
                onClose(tab.kind, tab.id);
              }}
            >
              <X size={12} strokeWidth={1.75} />
            </button>
          </div>
        );
      })}
    </div>
  );
}
