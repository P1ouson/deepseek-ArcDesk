import { X } from "lucide-react";
import { useT } from "../lib/i18n";
import type { TabAttention } from "../lib/tabSessionActivity";
import { Tooltip } from "./Tooltip";

export interface OpenTabsBarProps {
  tabs: TabAttention[];
  activeTabId?: string;
  onSelectTab: (tabId: string) => void;
  onCloseTab: (tabId: string) => void;
}

export function OpenTabsBar({ tabs, activeTabId, onSelectTab, onCloseTab }: OpenTabsBarProps) {
  const t = useT();
  if (tabs.length === 0) return null;

  return (
    <div className="open-tabs-bar wails-no-drag" role="tablist" aria-label={t("openTabs.label")}>
      {tabs.map((tab) => {
        const active = tab.tabId === activeTabId;
        const title = tab.topicTitle;
        const hint = tab.workspaceName && tab.workspaceName !== title ? `${title} · ${tab.workspaceName}` : title;
        return (
          <div
            key={tab.tabId}
            data-open-tab-id={tab.tabId}
            className={[
              "open-tabs-bar__item",
              active ? "open-tabs-bar__item--active" : "",
              tab.needsDecision ? "open-tabs-bar__item--decision" : "",
            ]
              .filter(Boolean)
              .join(" ")}
            role="presentation"
          >
            <button
              type="button"
              className="open-tabs-bar__select"
              role="tab"
              aria-selected={active}
              title={hint}
              onClick={() => onSelectTab(tab.tabId)}
            >
              <span className="open-tabs-bar__label">{title}</span>
              <span
                className={[
                  "open-tabs-bar__pulse",
                  tab.pulse === "running" ? "open-tabs-bar__pulse--running" : "open-tabs-bar__pulse--completed",
                ].join(" ")}
                aria-hidden="true"
              />
            </button>
            <Tooltip label={t("openTabs.close")}>
              <button
                type="button"
                className="open-tabs-bar__close"
                aria-label={t("openTabs.closeNamed", { title })}
                onClick={(event) => {
                  event.stopPropagation();
                  onCloseTab(tab.tabId);
                }}
              >
                <X size={12} />
              </button>
            </Tooltip>
          </div>
        );
      })}
    </div>
  );
}
