import { Pencil } from "lucide-react";
import type { KeyboardEvent } from "react";
import { useT } from "../lib/i18n";
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
  onHubPress?: (hub: import("../lib/dockHubs").DockHub) => void;
  onOpenDockTab?: (tab: RightDockTab) => void;
  onOpenPreviewMode?: (mode: import("../lib/dockHubs").PreviewMode) => void;
}

export function Topbar({
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
    <header className="studio-header wails-drag">
      <div className="studio-header__main wails-no-drag">
        <div className="studio-header__title-row">
          {editing ? (
            <input
              autoFocus
              className="studio-header__title-input"
              value={titleDraft}
              onChange={(e) => onTitleDraftChange(e.target.value)}
              onKeyDown={onTitleKeyDown}
              onBlur={onCommitRename}
              aria-label={t("topicBar.renameSession")}
            />
          ) : (
            <h1 className="studio-header__title">{title}</h1>
          )}
          {!editing && (
            <Tooltip label={t("topicBar.renameSession")}>
              <button type="button" className="studio-header__rename" onClick={onStartRename} aria-label={t("topicBar.renameSession")}>
                <Pencil size={12} />
              </button>
            </Tooltip>
          )}
        </div>
        <div className="studio-header__meta">
          <span className="studio-header__workspace" title={workspacePath}>
            {workspacePath}
          </span>
          {running && <span className="studio-header__pill studio-header__pill--running">{t("status.running")}</span>}
          {goalLabel ? (
            <span className="studio-header__pill" title={goalLabel}>
              {goalLabel.length > 28 ? `${goalLabel.slice(0, 28)}…` : goalLabel}
            </span>
          ) : null}
        </div>
      </div>

      <div className="studio-header__aside wails-no-drag">
        {sideConversationCount > 0 && (
          <span className="studio-header__badge" aria-label={t("sideChat.badge", { count: sideConversationCount })}>
            {sideConversationCount}
          </span>
        )}
      </div>
    </header>
  );
}
