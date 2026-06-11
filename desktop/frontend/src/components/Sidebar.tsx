import type { ReactNode } from "react";
import {
  CalendarClock,
  ChevronLeft,
  Code2,
  FileText,
  FolderOpen,
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Radio,
  Settings,
  Sparkles,
} from "lucide-react";
import logo from "../assets/logo.svg";
import { useT } from "../lib/i18n";
import type { AppMode } from "../lib/appMode";
import type { TabMeta } from "../lib/types";
import type { WriteTurn } from "../lib/writeConversation";
import { ProjectTree } from "./ProjectTree";
import { WriteConversationThread } from "./WriteConversationThread";

export interface SidebarProps {
  drawerOpen: boolean;
  onCloseDrawer: () => void;
  onToggleDrawer: () => void;
  appMode: AppMode;
  activeTab?: TabMeta;
  projectRevision: number;
  onOpenTopic: (scope: string, workspaceRoot: string, topicId: string) => void;
  onOpenWorkspace: () => void;
  onNewChat: () => void;
  onNewFreshChat?: () => void;
  onModeChange: (mode: AppMode) => void;
  onOpenSdd: () => void;
  onAddProject: () => Promise<void>;
  onOpenProjectHistory: (scope: "global" | "project", workspaceRoot: string) => void;
  onTopicsChanged?: () => Promise<void>;
  writeConversation?: WriteTurn[];
  writeRunning?: boolean;
}

function LabeledRailButton({
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
    <button
      type="button"
      className={`studio-rail__btn studio-rail__btn--labeled${active ? " studio-rail__btn--active" : ""}`}
      onClick={onClick}
      aria-label={label}
      aria-pressed={active}
    >
      <span className="studio-rail__btn-icon" aria-hidden="true">
        {icon}
      </span>
      <span className="studio-rail__btn-label">{label}</span>
    </button>
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
  drawerOpen,
  onCloseDrawer,
  onToggleDrawer,
  appMode,
  activeTab,
  projectRevision,
  onOpenTopic,
  onOpenWorkspace,
  onNewChat,
  onNewFreshChat,
  onModeChange,
  onOpenSdd,
  onAddProject,
  onOpenProjectHistory,
  onTopicsChanged,
  writeConversation = [],
  writeRunning = false,
}: SidebarProps) {
  const t = useT();
  const writeMode = appMode === "write";

  return (
    <div className={`studio-sidebar-shell${drawerOpen ? " studio-sidebar-shell--drawer-open" : ""}`}>
      <nav className="studio-rail wails-no-drag" aria-label={t("sidebar.navigation")}>
        <img src={logo} className="studio-rail__logo" alt="ArcDesk" />

        <LabeledRailButton
          icon={<Code2 size={17} />}
          label={t("modes.code")}
          active={!writeMode && appMode === "code"}
          onClick={() => onModeChange("code")}
        />
        <LabeledRailButton
          icon={<FileText size={17} />}
          label={t("modes.write")}
          active={writeMode}
          onClick={() => onModeChange("write")}
        />

        <span className="studio-rail__divider" aria-hidden="true" />

        <LabeledRailButton
          icon={drawerOpen ? <PanelLeftClose size={17} /> : <PanelLeftOpen size={17} />}
          label={drawerOpen ? t("sidebar.collapseDrawer") : t("sidebar.expandDrawer")}
          active={drawerOpen}
          onClick={onToggleDrawer}
        />
        <LabeledRailButton
          icon={<FolderOpen size={17} />}
          label={t("sidebar.importWorkspace")}
          onClick={onOpenWorkspace}
        />

        <span className="studio-rail__spacer" />

        <LabeledRailButton
          icon={<Sparkles size={17} />}
          label={t("sidebar.rail.extensions")}
          active={appMode === "plugins"}
          onClick={() => onModeChange("plugins")}
        />
        <LabeledRailButton
          icon={<CalendarClock size={17} />}
          label={t("sidebar.rail.schedule")}
          active={appMode === "schedule"}
          onClick={() => onModeChange("schedule")}
        />
        <LabeledRailButton
          icon={<Radio size={17} />}
          label={t("sidebar.rail.phone")}
          active={appMode === "phone"}
          onClick={() => onModeChange("phone")}
        />
        <LabeledRailButton
          icon={<Settings size={17} />}
          label={t("sidebar.rail.settings")}
          active={appMode === "settings"}
          onClick={() => onModeChange("settings")}
        />
      </nav>

      {drawerOpen ? (
        <aside className="studio-drawer wails-no-drag" aria-label={writeMode ? t("write.conversationDrawer") : t("sidebar.projects")}>
          <div className="studio-drawer__head">
            <h2 className="studio-drawer__title">{writeMode ? t("write.conversationDrawer") : t("sidebar.projects")}</h2>
            <button type="button" className="studio-drawer__collapse-btn" onClick={onCloseDrawer}>
              <ChevronLeft size={14} aria-hidden="true" />
              <span>{t("sidebar.collapseDrawer")}</span>
            </button>
          </div>

          <div className="studio-drawer__actions">
            <DrawerAction icon={<FolderOpen size={15} />} label={t("sidebar.importWorkspace")} onClick={onOpenWorkspace} />
            <DrawerAction icon={<Plus size={15} />} label={t("sidebar.newSession")} onClick={onNewChat} />
            {onNewFreshChat ? (
              <DrawerAction icon={<Plus size={15} />} label={t("sidebar.newFreshSession")} onClick={onNewFreshChat} />
            ) : null}
            {!writeMode ? (
              <DrawerAction icon={<Sparkles size={15} />} label={t("sidebar.newRequirement")} onClick={onOpenSdd} />
            ) : null}
          </div>

          {writeMode ? (
            <div className="studio-drawer__write-conversation">
              <div className="studio-drawer__section-label">{t("write.conversationSidebarHint")}</div>
              <WriteConversationThread turns={writeConversation} running={writeRunning} variant="sidebar" />
            </div>
          ) : (
            <div className="studio-drawer__tree">
              <ProjectTree
                variant="sidebar"
                activeScope={activeTab?.scope}
                activeWorkspaceRoot={activeTab?.workspaceRoot}
                activeTopicId={activeTab?.topicId}
                refreshSignal={projectRevision}
                onOpenTopic={onOpenTopic}
                onOpenProjectHistory={onOpenProjectHistory}
                onAddProject={onAddProject}
                onTopicsChanged={onTopicsChanged}
              />
            </div>
          )}
        </aside>
      ) : null}
    </div>
  );
}
