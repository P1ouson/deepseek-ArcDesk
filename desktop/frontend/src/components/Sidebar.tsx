import type { ReactNode } from "react";
import {
  CalendarClock,
  Code2,
  FileText,
  MessageCircle,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Puzzle,
  Settings,
  Sparkles,
} from "lucide-react";
import { useT } from "../lib/i18n";
import type { AppMode } from "../lib/appMode";
import type { TabMeta } from "../lib/types";
import { ProjectTree } from "./ProjectTree";
import { Tooltip } from "./Tooltip";

export interface SidebarProps {
  collapsed: boolean;
  onToggleCollapse: () => void;
  appMode: AppMode;
  activeTab?: TabMeta;
  projectRevision: number;
  currentWorkspaceName?: string;
  onOpenTopic: (scope: string, workspaceRoot: string, topicId: string) => void;
  onNewChat: () => void;
  onModeChange: (mode: AppMode) => void;
  onOpenSettings: () => void;
  onOpenSdd: () => void;
  onAddProject: () => Promise<void>;
  onUseCurrentProject?: () => Promise<void>;
  onOpenProjectHistory: (scope: "global" | "project", workspaceRoot: string) => void;
  onTopicsChanged?: () => Promise<void>;
}

function ActionRow({
  icon,
  label,
  onClick,
}: {
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <button type="button" className="sidebar__action" onClick={onClick}>
      <span className="sidebar__action-icon">{icon}</span>
      <span>{label}</span>
    </button>
  );
}

export function Sidebar({
  collapsed,
  onToggleCollapse,
  appMode,
  activeTab,
  projectRevision,
  currentWorkspaceName,
  onOpenTopic,
  onNewChat,
  onModeChange,
  onOpenSettings,
  onOpenSdd,
  onAddProject,
  onUseCurrentProject,
  onOpenProjectHistory,
  onTopicsChanged,
}: SidebarProps) {
  const t = useT();
  const writeMode = appMode === "write";

  if (collapsed) {
    return (
      <aside className="sidebar sidebar--collapsed-rail wails-no-drag" aria-label={t("sidebar.navigation")}>
        <Tooltip label={t("sidebar.expand")}>
          <button
            type="button"
            className="sidebar__rail-btn"
            onClick={onToggleCollapse}
            aria-label={t("sidebar.expand")}
          >
            <PanelLeftOpen size={16} />
          </button>
        </Tooltip>
      </aside>
    );
  }

  return (
    <aside className="sidebar wails-no-drag" aria-label={t("sidebar.navigation")}>
      <div className="sidebar__head">
        <div className="sidebar__mode-toggle" role="tablist" aria-label={t("sidebar.modeToggle")}>
          <button
            type="button"
            role="tab"
            aria-selected={!writeMode}
            className={`sidebar__mode-btn${!writeMode ? " sidebar__mode-btn--active" : ""}`}
            onClick={() => onModeChange("code")}
          >
            <Code2 size={14} />
            <span>{t("modes.code")}</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={writeMode}
            className={`sidebar__mode-btn${writeMode ? " sidebar__mode-btn--active" : ""}`}
            onClick={() => onModeChange("write")}
          >
            <FileText size={14} />
            <span>{t("modes.write")}</span>
          </button>
        </div>
        <Tooltip label={t("sidebar.collapse")}>
          <button
            type="button"
            className="sidebar__collapse-btn"
            onClick={onToggleCollapse}
            aria-label={t("sidebar.collapse")}
          >
            <PanelLeftClose size={15} />
          </button>
        </Tooltip>
      </div>

      <nav className="sidebar__actions" aria-label={t("sidebar.quickActions")}>
        <ActionRow icon={<Plus size={15} />} label={t("topbar.newSession")} onClick={onNewChat} />
        <ActionRow icon={<Sparkles size={15} />} label={t("sidebar.newRequirement")} onClick={onOpenSdd} />
        <ActionRow icon={<Puzzle size={15} />} label={t("sidebar.plugins")} onClick={() => onModeChange("plugins")} />
        <ActionRow icon={<CalendarClock size={15} />} label={t("modes.schedule")} onClick={() => onModeChange("schedule")} />
      </nav>

      <div className="sidebar__tree">
        <ProjectTree
          variant="sidebar"
          activeScope={activeTab?.scope}
          activeWorkspaceRoot={activeTab?.workspaceRoot}
          activeTopicId={activeTab?.topicId}
          currentWorkspaceName={currentWorkspaceName}
          refreshSignal={projectRevision}
          onOpenTopic={onOpenTopic}
          onOpenProjectHistory={onOpenProjectHistory}
          onAddProject={onAddProject}
          onUseCurrentProject={onUseCurrentProject}
          onTopicsChanged={onTopicsChanged}
        />
      </div>

      <footer className="sidebar__footer">
        <ActionRow icon={<MessageCircle size={15} />} label={t("modes.phone")} onClick={() => onModeChange("phone")} />
        <ActionRow icon={<Settings size={15} />} label={t("topbar.settings")} onClick={onOpenSettings} />
      </footer>
    </aside>
  );
}
