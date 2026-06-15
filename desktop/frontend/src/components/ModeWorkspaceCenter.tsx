import { lazy, Suspense, useCallback, useEffect, useState } from "react";
import type { AppMode } from "../lib/appMode";
import type { ComposerWriteContext, KnowledgeView } from "../lib/types";
import type { WriteTurn } from "../lib/writeConversation";
import type { RightDockTab } from "./Topbar";
import { ConnectPhoneView } from "./ConnectPhoneView";
import { ScheduleTasksView } from "./ScheduleTasksView";
import { WriteSidebar } from "./WriteSidebar";
import { WriteWorkspaceView } from "./WriteWorkspaceView";
import { useT } from "../lib/i18n";

const PluginMarketplace = lazy(() =>
  import("./PluginMarketplace").then((module) => ({ default: module.PluginMarketplace })),
);
const SettingsPage = lazy(() =>
  import("./SettingsPanel").then((module) => ({ default: module.SettingsPage })),
);
const KnowledgePanel = lazy(() =>
  import("./KnowledgePanel").then((module) => ({ default: module.KnowledgePanel })),
);

function ModeCenterFallback() {
  const t = useT();
  return <div className="mode-center mode-center--loading">{t("common.loading")}</div>;
}

export interface ModeWorkspaceCenterProps {
  mode: AppMode;
  workspaceRoot: string;
  activeTabId?: string;
  activeTabLabel?: string;
  activeWorkspaceName?: string;
  writeWorkspaceRoot?: string;
  writeSelectedFile?: string;
  onWriteSelectedFileChange?: (path: string) => void;
  onWriteWorkspaceChange?: (root: string) => void;
  onPrompt?: (text: string) => void;
  onComposerPrompt?: (text: string) => void;
  onDraftComposer?: (context: ComposerWriteContext) => void;
  onPickWriteWorkspace?: () => Promise<string | undefined>;
  onPickWriteFile?: () => Promise<string | undefined>;
  onFilesChanged?: () => void;
  writeConversation?: WriteTurn[];
  writeAgentRunning?: boolean;
  rightPanelOpen?: boolean;
  onToggleRightPanel?: () => void;
  onSettingsChanged?: () => void;
  onOpenHistory?: () => void;
  onOpenMemory?: () => void;
  onOpenCapabilities?: () => void;
  onOpenTrash?: () => void;
  onConfigureProjectSandbox?: () => void;
  onModeChange?: (mode: AppMode) => void;
  onOpenDockTab?: (tab: RightDockTab) => void;
  onOpenTerminal?: () => void;
  onOpenOnboarding?: () => void;
  knowledgeView?: KnowledgeView | null;
  onKnowledgeConfirm?: (id: string) => Promise<void> | void;
  onKnowledgeStale?: (id: string) => Promise<void> | void;
}

function WriteModeWorkspace({
  writeWorkspaceRoot,
  writeSelectedFile,
  onWriteSelectedFileChange,
  onWriteWorkspaceChange,
  onDraftComposer,
  onPickWriteWorkspace,
  onPickWriteFile,
  onFilesChanged,
  writeConversation,
  writeAgentRunning,
  rightPanelOpen,
  onToggleRightPanel,
}: {
  writeWorkspaceRoot: string;
  writeSelectedFile?: string;
  onWriteSelectedFileChange?: (path: string) => void;
  onWriteWorkspaceChange?: (root: string) => void;
  onDraftComposer?: (context: ComposerWriteContext) => void;
  onPickWriteWorkspace?: () => Promise<string | undefined>;
  onPickWriteFile?: () => Promise<string | undefined>;
  onFilesChanged?: () => void;
  writeConversation?: WriteTurn[];
  writeAgentRunning?: boolean;
  rightPanelOpen?: boolean;
  onToggleRightPanel?: () => void;
}) {
  const [selectedPath, setSelectedPath] = useState<string | undefined>(writeSelectedFile);
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setSelectedPath(writeSelectedFile);
  }, [writeSelectedFile]);

  useEffect(() => {
    if (!writeSelectedFile) {
      setSelectedPath(undefined);
      setDirty(false);
    }
  }, [writeWorkspaceRoot, writeSelectedFile]);

  const handleSelectFile = useCallback(
    (path: string) => {
      setSelectedPath(path || undefined);
      onWriteSelectedFileChange?.(path);
    },
    [onWriteSelectedFileChange],
  );

  return (
    <div className="mode-center mode-center--write write-studio-shell">
      <WriteSidebar
        workspaceRoot={writeWorkspaceRoot}
        selectedPath={selectedPath}
        dirty={dirty}
        onSelectFile={handleSelectFile}
        onPickWorkspace={onPickWriteWorkspace}
        onPickFile={onPickWriteFile}
        onWorkspaceChange={onWriteWorkspaceChange}
        onFilesChanged={onFilesChanged}
      />
      <WriteWorkspaceView
        filePath={selectedPath}
        onSaved={onFilesChanged}
        onFilePathChange={handleSelectFile}
        onDraftComposer={onDraftComposer}
        onDirtyChange={setDirty}
        conversationTurns={writeConversation}
        agentRunning={writeAgentRunning}
        rightPanelOpen={rightPanelOpen}
        onToggleRightPanel={onToggleRightPanel}
      />
    </div>
  );
}

export function ModeWorkspaceCenter({
  mode,
  workspaceRoot,
  activeTabId,
  activeTabLabel,
  activeWorkspaceName,
  writeWorkspaceRoot = "",
  writeSelectedFile,
  onWriteSelectedFileChange,
  onWriteWorkspaceChange,
  onDraftComposer,
  onPickWriteWorkspace,
  onPickWriteFile,
  onFilesChanged,
  writeConversation,
  writeAgentRunning,
  rightPanelOpen,
  onToggleRightPanel,
  onComposerPrompt,
  onSettingsChanged,
  onOpenHistory,
  onOpenMemory,
  onOpenCapabilities,
  onOpenTrash,
  onConfigureProjectSandbox,
  onModeChange,
  onOpenDockTab,
  onOpenTerminal,
  onOpenOnboarding,
  knowledgeView = null,
  onKnowledgeConfirm,
  onKnowledgeStale,
}: ModeWorkspaceCenterProps) {
  switch (mode) {
    case "write":
      return (
        <WriteModeWorkspace
          writeWorkspaceRoot={writeWorkspaceRoot}
          writeSelectedFile={writeSelectedFile}
          onWriteSelectedFileChange={onWriteSelectedFileChange}
          onWriteWorkspaceChange={onWriteWorkspaceChange}
          onDraftComposer={onDraftComposer}
          onPickWriteWorkspace={onPickWriteWorkspace}
          onPickWriteFile={onPickWriteFile}
          onFilesChanged={onFilesChanged}
          writeConversation={writeConversation}
          writeAgentRunning={writeAgentRunning}
          rightPanelOpen={rightPanelOpen}
          onToggleRightPanel={onToggleRightPanel}
        />
      );
    case "phone":
      return (
        <ConnectPhoneView
          workspaceRoot={workspaceRoot}
          activeTabId={activeTabId}
          tabLabel={activeTabLabel}
          workspaceName={activeWorkspaceName}
        />
      );
    case "schedule":
      return <ScheduleTasksView workspaceRoot={workspaceRoot} />;
    case "knowledge":
      return (
        <Suspense fallback={<ModeCenterFallback />}>
          <KnowledgePanel
            presentation="page"
            view={knowledgeView}
            onConfirm={onKnowledgeConfirm ?? (async () => {})}
            onStale={onKnowledgeStale ?? (async () => {})}
          />
        </Suspense>
      );
    case "plugins":
      return (
        <Suspense fallback={<ModeCenterFallback />}>
          <PluginMarketplace />
        </Suspense>
      );
    case "settings":
      return (
        <Suspense fallback={<ModeCenterFallback />}>
          <SettingsPage
          onComposerPrompt={onComposerPrompt}
          onChanged={() => onSettingsChanged?.()}
          onOpenHistory={onOpenHistory}
          onOpenMemory={onOpenMemory}
          onOpenCapabilities={onOpenCapabilities}
          onOpenTrash={onOpenTrash}
          onConfigureProjectSandbox={onConfigureProjectSandbox}
          onModeChange={onModeChange}
          onOpenDockTab={onOpenDockTab}
          onOpenTerminal={onOpenTerminal}
          onOpenOnboarding={onOpenOnboarding}
          />
        </Suspense>
      );
    default:
      return null;
  }
}
