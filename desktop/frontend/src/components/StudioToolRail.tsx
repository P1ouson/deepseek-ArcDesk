import { DockHubButtons } from "./DockHubButtons";
import type { RightDockTab } from "./Topbar";
import type { DockHub, PreviewMode } from "../lib/dockHubs";

export interface StudioToolRailProps {
  dockOpen: boolean;
  activeDockTab?: RightDockTab | null;
  terminalOpen?: boolean;
  onHubPress: (hub: DockHub) => void;
  onOpenDockTab: (tab: RightDockTab) => void;
  onOpenPreviewMode: (mode: PreviewMode) => void;
}

export function StudioToolRail({
  dockOpen,
  activeDockTab,
  terminalOpen,
  onHubPress,
  onOpenDockTab,
  onOpenPreviewMode,
}: StudioToolRailProps) {
  return (
    <aside className="studio-tool-rail wails-no-drag" aria-label="Tools">
      <DockHubButtons
        dockOpen={dockOpen}
        activeDockTab={activeDockTab}
        terminalOpen={terminalOpen}
        onHubPress={onHubPress}
        onOpenDockTab={onOpenDockTab}
        onOpenPreviewMode={onOpenPreviewMode}
      />
    </aside>
  );
}
