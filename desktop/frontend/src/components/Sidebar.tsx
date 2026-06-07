import type { ReactNode } from "react";
import {
  CalendarClock,
  Code2,
  FileText,
  FolderKanban,
  MessageCircle,
  Plus,
  Puzzle,
  Settings,
  Sparkles,
  X,
} from "lucide-react";
import logo from "../assets/logo.svg";
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

function RailButton({
  icon,
  label,
  active,
  onClick,
}: {
  icon: ReactNode;
  label: string;
  active?: boolean;
  onClick: () => void;
}) {
  return (
    <Tooltip label={label}>
      <button
        type="button"
        className={`studio-rail__btn${active ? " studio-rail__btn--active" : ""}`}
        onClick={onClick}
        aria-label={label}
        aria-pressed={active}
      >
        {icon}
      </button>
    </Tooltip>
  );
}

function DrawerAction({
  icon,
  label,
  onClick,
}: {
  icon: ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <button type="button" className="studio-drawer__action" onClick={onClick}>
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
  const drawerOpen = !collapsed;

  return (
    <>
      <nav className="studio-rail wails-no-drag" aria-label={t("sidebar.navigation")}>
        <img src={logo} className="studio-rail__logo" alt="Reasonix" />

        <RailButton
          icon={<FolderKanban size={18} />}
          label={drawerOpen ? t("sidebar.collapse") : t("sidebar.expand")}
          active={drawerOpen}
          onClick={onToggleCollapse}
        />

        <RailButton icon={<Plus size={18} />} label={t("topbar.newSession")} onClick={onNewChat} />

        <span className="studio-rail__divider" aria-hidden="true" />

        <RailButton
          icon={<Code2 size={18} />}
          label={t("modes.code")}
          active={!writeMode && appMode === "code"}
          onClick={() => onModeChange("code")}
        />
        <RailButton
          icon={<FileText size={18} />}
          label={t("modes.write")}
          active={writeMode}
          onClick={() => onModeChange("write")}
        />

        <span className="studio-rail__spacer" />

        <RailButton icon={<Puzzle size={18} />} label={t("sidebar.plugins")} onClick={() => onModeChange("plugins")} />
        <RailButton icon={<CalendarClock size={18} />} label={t("modes.schedule")} onClick={() => onModeChange("schedule")} />
        <RailButton icon={<MessageCircle size={18} />} label={t("modes.phone")} onClick={() => onModeChange("phone")} />
        <RailButton icon={<Settings size={18} />} label={t("topbar.settings")} onClick={onOpenSettings} />
      </nav>

      <button
        type="button"
        className={`studio-drawer-backdrop${drawerOpen ? " studio-drawer-backdrop--open" : ""}`}
        aria-label={t("sidebar.collapse")}
        onClick={onToggleCollapse}
        tabIndex={drawerOpen ? 0 : -1}
      />

      <aside
        className={`studio-drawer wails-no-drag${drawerOpen ? " studio-drawer--open" : ""}`}
        aria-label={t("sidebar.projects")}
        aria-hidden={!drawerOpen}
      >
        <div className="studio-drawer__head">
          <h2 className="studio-drawer__title">{t("sidebar.projects")}</h2>
          <button type="button" className="studio-drawer__close" onClick={onToggleCollapse} aria-label={t("sidebar.collapse")}>
            <X size={14} />
          </button>
        </div>

        <div className="studio-drawer__mode" role="tablist" aria-label={t("sidebar.modeToggle")}>
          <button
            type="button"
            role="tab"
            aria-selected={!writeMode}
            className={`studio-drawer__mode-btn${!writeMode ? " studio-drawer__mode-btn--active" : ""}`}
            onClick={() => onModeChange("code")}
          >
            <Code2 size={14} />
            <span>{t("modes.code")}</span>
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={writeMode}
            className={`studio-drawer__mode-btn${writeMode ? " studio-drawer__mode-btn--active" : ""}`}
            onClick={() => onModeChange("write")}
          >
            <FileText size={14} />
            <span>{t("modes.write")}</span>
          </button>
        </div>

        <div className="studio-drawer__actions">
          <DrawerAction icon={<Plus size={15} />} label={t("topbar.newSession")} onClick={onNewChat} />
          <DrawerAction icon={<Sparkles size={15} />} label={t("sidebar.newRequirement")} onClick={onOpenSdd} />
        </div>

        <div className="studio-drawer__tree">
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
      </aside>
    </>
  );
}
