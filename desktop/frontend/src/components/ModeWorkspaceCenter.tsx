import { useEffect, useState } from "react";
import type { AppMode } from "../lib/appMode";
import type { ComposerWriteContext } from "../lib/types";
import type { WriteTurn } from "../lib/writeConversation";
import type { RightDockTab } from "./Topbar";
import { ConnectPhoneView } from "./ConnectPhoneView";
import { PluginMarketplace } from "./PluginMarketplace";
import { ScheduleTasksView } from "./ScheduleTasksView";
import { SettingsPage } from "./SettingsPanel";
import { WriteSidebar } from "./WriteSidebar";
import { WriteWorkspaceView } from "./WriteWorkspaceView";

export interface ModeWorkspaceCenterProps {
  mode: AppMode;
  workspaceRoot: string;
  activeTabId?: string;
  activeTabLabel?: string;
  activeWorkspaceName?: string;
  writeWorkspaceRoot?: string;
  onWriteWorkspaceChange?: (root: string) => void;
  onPrompt?: (text: string) => void;
  onDraftComposer?: (context: ComposerWriteContext) => void;
  onPickWriteWorkspace?: () => Promise<string | undefined>;
  onFilesChanged?: () => void;
  writeConversation?: WriteTurn[];
  writeAgentRunning?: boolean;
  onSettingsChanged?: () => void;
  onOpenHistory?: () => void;
  onOpenMemory?: () => void;
  onOpenCapabilities?: () => void;
  onOpenTrash?: () => void;
  onConfigureProjectSandbox?: () => void;
  onModeChange?: (mode: AppMode) => void;
  onOpenDockTab?: (tab: RightDockTab) => void;
  onOpenTerminal?: () => void;
}

function WriteModeWorkspace({
  writeWorkspaceRoot,
  onWriteWorkspaceChange,
  onDraftComposer,
  onPickWriteWorkspace,
  onFilesChanged,
  writeConversation,
  writeAgentRunning,
}: {
  writeWorkspaceRoot: string;
  onWriteWorkspaceChange?: (root: string) => void;
  onDraftComposer?: (context: ComposerWriteContext) => void;
  onPickWriteWorkspace?: () => Promise<string | undefined>;
  onFilesChanged?: () => void;
  writeConversation?: WriteTurn[];
  writeAgentRunning?: boolean;
}) {
  const [selectedPath, setSelectedPath] = useState<string | undefined>();
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setSelectedPath(undefined);
    setDirty(false);
  }, [writeWorkspaceRoot]);

  return (
    <div className="mode-center mode-center--write write-studio-shell">
      <WriteSidebar
        workspaceRoot={writeWorkspaceRoot}
        selectedPath={selectedPath}
        dirty={dirty}
        onSelectFile={setSelectedPath}
        onPickWorkspace={onPickWriteWorkspace}
        onWorkspaceChange={onWriteWorkspaceChange}
        onFilesChanged={onFilesChanged}
      />
      <WriteWorkspaceView
        filePath={selectedPath}
        onSaved={onFilesChanged}
        onFilePathChange={setSelectedPath}
        onDraftComposer={onDraftComposer}
        onDirtyChange={setDirty}
        conversationTurns={writeConversation}
        agentRunning={writeAgentRunning}
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
  onWriteWorkspaceChange,
  onDraftComposer,
  onPickWriteWorkspace,
  onFilesChanged,
  writeConversation,
  writeAgentRunning,
  onSettingsChanged,
  onOpenHistory,
  onOpenMemory,
  onOpenCapabilities,
  onOpenTrash,
  onConfigureProjectSandbox,
  onModeChange,
  onOpenDockTab,
  onOpenTerminal,
}: ModeWorkspaceCenterProps) {
  switch (mode) {
    case "write":
      return (
        <WriteModeWorkspace
          writeWorkspaceRoot={writeWorkspaceRoot}
          onWriteWorkspaceChange={onWriteWorkspaceChange}
          onDraftComposer={onDraftComposer}
          onPickWriteWorkspace={onPickWriteWorkspace}
          onFilesChanged={onFilesChanged}
          writeConversation={writeConversation}
          writeAgentRunning={writeAgentRunning}
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
    case "plugins":
      return <PluginMarketplace />;
    case "settings":
      return (
        <SettingsPage
          workspaceRoot={workspaceRoot}
          onChanged={() => onSettingsChanged?.()}
          onOpenHistory={onOpenHistory}
          onOpenMemory={onOpenMemory}
          onOpenCapabilities={onOpenCapabilities}
          onOpenTrash={onOpenTrash}
          onConfigureProjectSandbox={onConfigureProjectSandbox}
          onModeChange={onModeChange}
          onOpenDockTab={onOpenDockTab}
          onOpenTerminal={onOpenTerminal}
        />
      );
    default:
      return null;
  }
}
