import { DockHubButtons } from "./DockHubButtons";
import type { RightDockTab } from "./Topbar";
import type { DockHub, PreviewMode } from "../lib/dockHubs";

export interface StudioToolRailProps {
  dockOpen: boolean;
  previewOpen?: boolean;
  activeDockTab?: RightDockTab | null;
  onHubPress: (hub: DockHub) => void;
  onOpenDockTab: (tab: RightDockTab) => void;
  onOpenPreviewMode: (mode: PreviewMode) => void;
}

export function StudioToolRail({
  dockOpen,
  previewOpen = false,
  activeDockTab,
  onHubPress,
  onOpenDockTab,
  onOpenPreviewMode,
}: StudioToolRailProps) {
  return (
    <aside className="studio-tool-rail wails-no-drag" aria-label="Tools">
      <DockHubButtons
        dockOpen={dockOpen}
        previewOpen={previewOpen}
        activeDockTab={activeDockTab}
        onHubPress={onHubPress}
        onOpenDockTab={onOpenDockTab}
        onOpenPreviewMode={onOpenPreviewMode}
      />
    </aside>
  );
}
