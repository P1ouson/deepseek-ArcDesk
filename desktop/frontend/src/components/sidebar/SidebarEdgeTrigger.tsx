import { PanelRightOpen } from "lucide-react";
import { useT } from "../../lib/i18n";
import { Tooltip } from "../Tooltip";

export function SidebarEdgeTrigger({ onOpen }: { onOpen: () => void }) {
  const t = useT();
  return (
    <Tooltip label={t("sidebar.openPanel")} side="left">
      <button
        type="button"
        className="sidebar-edge-trigger wails-no-drag"
        onClick={onOpen}
        aria-label={t("sidebar.openPanel")}
      >
        <PanelRightOpen size={16} strokeWidth={1.75} />
      </button>
    </Tooltip>
  );
}
