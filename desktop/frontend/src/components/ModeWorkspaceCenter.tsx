import { useEffect, useState } from "react";
import type { AppMode } from "../lib/appMode";
import { ConnectPhoneView } from "./ConnectPhoneView";
import { PluginMarketplace } from "./PluginMarketplace";
import { ScheduleTasksView } from "./ScheduleTasksView";
import { WriteSidebar } from "./WriteSidebar";
import { WriteWorkspaceView } from "./WriteWorkspaceView";

export interface ModeWorkspaceCenterProps {
  mode: AppMode;
  workspaceRoot: string;
  onPrompt?: (text: string) => void;
  onFilesChanged?: () => void;
}

function WriteModeWorkspace({
  workspaceRoot,
  onPrompt,
  onFilesChanged,
}: {
  workspaceRoot: string;
  onPrompt?: (text: string) => void;
  onFilesChanged?: () => void;
}) {
  const [root, setRoot] = useState(workspaceRoot);
  const [selectedPath, setSelectedPath] = useState<string | undefined>();

  useEffect(() => {
    setRoot(workspaceRoot);
  }, [workspaceRoot]);

  return (
    <div className="mode-center mode-center--write">
      <WriteSidebar
        workspaceRoot={root}
        selectedPath={selectedPath}
        onSelectFile={setSelectedPath}
        onWorkspaceChange={setRoot}
        onFilesChanged={onFilesChanged}
      />
      <WriteWorkspaceView filePath={selectedPath} onSaved={onFilesChanged} onPrompt={onPrompt} />
    </div>
  );
}

export function ModeWorkspaceCenter({ mode, workspaceRoot, onPrompt, onFilesChanged }: ModeWorkspaceCenterProps) {
  switch (mode) {
    case "write":
      return <WriteModeWorkspace workspaceRoot={workspaceRoot} onPrompt={onPrompt} onFilesChanged={onFilesChanged} />;
    case "phone":
      return (
        <div className="mode-center mode-center--phone">
          <ConnectPhoneView workspaceRoot={workspaceRoot} />
        </div>
      );
    case "schedule":
      return (
        <div className="mode-center mode-center--schedule">
          <ScheduleTasksView workspaceRoot={workspaceRoot} />
        </div>
      );
    case "plugins":
      return (
        <div className="mode-center mode-center--plugins">
          <PluginMarketplace />
        </div>
      );
    default:
      return null;
  }
}
