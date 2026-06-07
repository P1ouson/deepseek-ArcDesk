import { PanelLeftOpen, Pencil } from "lucide-react";
import type { KeyboardEvent } from "react";
import { useT } from "../lib/i18n";
import type { DockHub, PreviewMode } from "../lib/dockHubs";
import { DockHubButtons } from "./DockHubButtons";
import { Tooltip } from "./Tooltip";

export type RightDockTab = "context" | "changes" | "todo" | "git" | "browser" | "files";

export interface TopbarProps {
  sidebarCollapsed: boolean;
  onToggleSidebar: () => void;
  title: string;
  workspacePath: string;
  editing: boolean;
  titleDraft: string;
  onTitleDraftChange: (value: string) => void;
  onStartRename: () => void;
  onCommitRename: () => void;
  onCancelRename: () => void;
  running: boolean;
  goalLabel?: string;
  sideConversationCount: number;
  dockOpen?: boolean;
  activeDockTab?: RightDockTab | null;
  terminalOpen?: boolean;
  onHubPress: (hub: DockHub) => void;
  onOpenDockTab: (tab: RightDockTab) => void;
  onOpenPreviewMode: (mode: PreviewMode) => void;
}

export function Topbar({
  sidebarCollapsed,
  onToggleSidebar,
  title,
  workspacePath,
  editing,
  titleDraft,
  onTitleDraftChange,
  onStartRename,
  onCommitRename,
  onCancelRename,
  running,
  goalLabel,
  sideConversationCount,
  dockOpen = false,
  activeDockTab,
  terminalOpen = false,
  onHubPress,
  onOpenDockTab,
  onOpenPreviewMode,
}: TopbarProps) {
  const t = useT();

  const onTitleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      event.preventDefault();
      onCommitRename();
    }
    if (event.key === "Escape") {
      event.preventDefault();
      onCancelRename();
    }
  };

  return (
    <header className="topbar wails-drag">
      <div className="topbar__left wails-no-drag">
        {sidebarCollapsed && (
          <Tooltip label={t("sidebar.expand")}>
            <button type="button" className="topbar__toggle topbar__toggle--expand" onClick={onToggleSidebar} aria-label={t("sidebar.expand")}>
              <PanelLeftOpen size={14} />
            </button>
          </Tooltip>
        )}
        <div className="topbar__stack">
          <div className="topbar__title-row">
            <div className="topbar__title-main">
              {editing ? (
                <input
                  autoFocus
                  className="topbar__title-input"
                  value={titleDraft}
                  onChange={(e) => onTitleDraftChange(e.target.value)}
                  onKeyDown={onTitleKeyDown}
                  onBlur={onCommitRename}
                />
              ) : (
                <h1 className="topbar__title">{title}</h1>
              )}
              {!editing && (
                <Tooltip label={t("topicBar.renameSession")}>
                  <button type="button" className="topbar__toggle" onClick={onStartRename} aria-label={t("topicBar.renameSession")}>
                    <Pencil size={12} />
                  </button>
                </Tooltip>
              )}
            </div>
            <div className="topbar__title-actions">
              {running && <span className="topbar__pill topbar__pill--running">Running</span>}
              {goalLabel ? (
                <span className="topbar__pill topbar__pill--goal" title={goalLabel}>
                  {goalLabel.length > 24 ? `${goalLabel.slice(0, 24)}…` : goalLabel}
                </span>
              ) : null}
              <div className="topbar__dock-tools wails-no-drag">
                <DockHubButtons
                  dockOpen={dockOpen}
                  activeDockTab={activeDockTab}
                  terminalOpen={terminalOpen}
                  onHubPress={onHubPress}
                  onOpenDockTab={onOpenDockTab}
                  onOpenPreviewMode={onOpenPreviewMode}
                />
              </div>
              {sideConversationCount > 0 && (
                <span className="topbar__side-badge" aria-label={t("sideChat.badge", { count: sideConversationCount })}>
                  {sideConversationCount}
                </span>
              )}
            </div>
          </div>
          <div className="topbar__bottom-row">
            <div className="topbar__workspace" title={workspacePath}>
              {workspacePath}
            </div>
          </div>
        </div>
      </div>
    </header>
  );
}
