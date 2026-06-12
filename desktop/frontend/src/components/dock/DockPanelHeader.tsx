import type { ReactNode } from "react";
import { RefreshCw } from "lucide-react";
import { Tooltip } from "../Tooltip";

export function DockPanelHeader({
  title,
  cwd,
  cwdLabel,
  refreshLabel,
  refreshing,
  onRefresh,
  meta,
}: {
  title: string;
  cwd?: string;
  cwdLabel: string;
  refreshLabel: string;
  refreshing?: boolean;
  onRefresh: () => void;
  meta?: ReactNode;
}) {
  return (
    <header className="dock-panel__head">
      <div className="dock-panel__head-main">
        <h2 className="dock-panel__title">{title}</h2>
        <Tooltip label={cwd ?? undefined}>
          <p className="dock-panel__meta">{meta ?? cwdLabel}</p>
        </Tooltip>
      </div>
      <Tooltip label={refreshLabel}>
        <button type="button" className="dock-panel__ghost" onClick={onRefresh} aria-label={refreshLabel}>
          <RefreshCw size={14} strokeWidth={1.75} className={refreshing ? "dock-panel__spin" : undefined} />
        </button>
      </Tooltip>
    </header>
  );
}
