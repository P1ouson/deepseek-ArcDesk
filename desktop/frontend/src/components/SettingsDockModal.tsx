import { ChangesPanel } from "./ChangesPanel";
import { FilesPanel } from "./FilesPanel";
import { GitPanel } from "./GitPanel";
import { dockTabLabel } from "./DockHubButtons";
import { StudioCenterModal } from "./StudioCenterModal";
import type { RightDockTab } from "./Topbar";
import { useT } from "../lib/i18n";

export function SettingsDockModal({
  tab,
  cwd,
  refreshKey = 0,
  onClose,
}: {
  tab: RightDockTab;
  cwd?: string;
  refreshKey?: number;
  onClose: () => void;
}) {
  const t = useT();
  if (tab !== "files" && tab !== "changes" && tab !== "git") return null;

  return (
    <StudioCenterModal title={dockTabLabel(tab, t)} onClose={onClose} wide className="settings-dock-modal-shell">
      <div className="settings-dock-modal">
        {tab === "files" ? <FilesPanel cwd={cwd} refreshKey={refreshKey} onOpenFile={() => {}} /> : null}
        {tab === "changes" ? <ChangesPanel cwd={cwd} refreshKey={refreshKey} running={false} /> : null}
        {tab === "git" ? <GitPanel cwd={cwd} refreshKey={refreshKey} /> : null}
      </div>
    </StudioCenterModal>
  );
}
