import { PanelRightClose, PanelRightOpen, Pencil } from "lucide-react";
import type { KeyboardEvent } from "react";
import { useT } from "../lib/i18n";
import { Tooltip } from "./Tooltip";

export type RightDockTab = "context" | "changes" | "todo" | "git" | "browser" | "page" | "files";

export interface TopbarProps {
  title: string;
  workspacePath?: string;
  editing: boolean;
  titleDraft: string;
  onTitleDraftChange: (value: string) => void;
  onStartRename: () => void;
  onCommitRename: () => void;
  onCancelRename: () => void;
  goalLabel?: string;
  sideConversationCount: number;
  onOpenSideConversation?: () => void;
  pendingDecisionLabel?: string;
  onFocusPendingDecision?: () => void;
  backgroundAttentionCount?: number;
  onFocusBackgroundSession?: () => void;
  showRightPanelToggle?: boolean;
  rightPanelOpen?: boolean;
  onToggleRightPanel?: () => void;
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
  goalLabel,
  sideConversationCount,
  onOpenSideConversation,
  pendingDecisionLabel,
  onFocusPendingDecision,
  backgroundAttentionCount = 0,
  onFocusBackgroundSession,
  showRightPanelToggle = false,
  rightPanelOpen = false,
  onToggleRightPanel,
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
          <div className="studio-header__meta-main">
            {workspacePath ? (
              <span className="studio-header__workspace" title={workspacePath}>
                {workspacePath}
              </span>
            ) : null}
            {goalLabel ? (
              <span className="studio-header__pill" title={goalLabel}>
                {goalLabel.length > 28 ? `${goalLabel.slice(0, 28)}…` : goalLabel}
              </span>
            ) : null}
          </div>
        </div>
      </div>

      <div className="studio-header__aside wails-no-drag">
        {showRightPanelToggle ? (
          <Tooltip
            label={rightPanelOpen ? t("sidebar.collapsePanel") : t("sidebar.openPanel")}
            side="bottom"
            block
          >
            <button
              type="button"
              className={`studio-header__right-panel-btn${rightPanelOpen ? " studio-header__right-panel-btn--open" : ""}`}
              onClick={() => onToggleRightPanel?.()}
              aria-label={rightPanelOpen ? t("sidebar.collapsePanel") : t("sidebar.openPanel")}
              aria-pressed={rightPanelOpen}
            >
              {rightPanelOpen ? <PanelRightClose size={16} strokeWidth={1.75} /> : <PanelRightOpen size={16} strokeWidth={1.75} />}
              <span>{rightPanelOpen ? t("sidebar.collapsePanel") : t("sidebar.expandPanel")}</span>
            </button>
          </Tooltip>
        ) : null}
        {backgroundAttentionCount > 0 && onFocusBackgroundSession ? (
          <button type="button" className="studio-header__decision-pill studio-header__decision-pill--muted" onClick={onFocusBackgroundSession}>
            {t("openTabs.backgroundBadge", { count: backgroundAttentionCount })}
          </button>
        ) : null}
        {pendingDecisionLabel && onFocusPendingDecision ? (
          <button type="button" className="studio-header__decision-pill" onClick={onFocusPendingDecision}>
            {pendingDecisionLabel}
          </button>
        ) : null}
        {sideConversationCount > 0 ? (
          onOpenSideConversation ? (
            <button
              type="button"
              className="studio-header__badge studio-header__badge--btn"
              aria-label={t("sideChat.badge", { count: sideConversationCount })}
              onClick={onOpenSideConversation}
            >
              {sideConversationCount}
            </button>
          ) : (
            <span className="studio-header__badge" aria-label={t("sideChat.badge", { count: sideConversationCount })}>
              {sideConversationCount}
            </span>
          )
        ) : null}
      </div>
    </header>
  );
}
