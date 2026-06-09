import { type CSSProperties, type ReactNode } from "react";
import { AlertTriangle, List, Zap } from "lucide-react";
import { useT } from "../lib/i18n";
import type { Mode } from "../lib/types";

const MODE_OPTIONS: Array<{ id: Mode; label: string; icon: ReactNode }> = [
  { id: "normal", label: "auto", icon: <Zap size={13} /> },
  { id: "plan", label: "plan", icon: <List size={13} /> },
  { id: "yolo", label: "yolo", icon: <AlertTriangle size={13} /> },
];

const MODE_SEGMENT_INDEX: Record<Mode, number> = {
  normal: 0,
  plan: 1,
  yolo: 2,
};

export function ComposerModeBar({
  mode,
  onSetMode,
  disabled,
  running,
  inline = false,
}: {
  mode: Mode;
  onSetMode: (mode: Mode) => void;
  disabled?: boolean;
  running: boolean;
  inline?: boolean;
}) {
  const t = useT();
  const bar = (
    <div
      className="composer-modebar motion-segment"
      role="toolbar"
      aria-label={t("composer.modeTitle")}
      style={
        {
          "--motion-segment-index": MODE_SEGMENT_INDEX[mode],
          "--motion-segment-count": MODE_OPTIONS.length,
        } as CSSProperties
      }
    >
      {MODE_OPTIONS.map((option) => (
        <button
          key={option.id}
          type="button"
          className={`composer-modebar__item composer-modebar__item--${option.id}${mode === option.id ? " composer-modebar__item--active" : ""}`}
          onClick={() => onSetMode(option.id)}
          aria-pressed={mode === option.id}
          disabled={disabled || running}
        >
          {option.icon}
          <span>{option.label}</span>
        </button>
      ))}
    </div>
  );
  if (inline) return bar;
  return <div className="composer-shell__cmdrow">{bar}</div>;
}

/** @deprecated Use ComposerModeBar */
export function ComposerModeToggle({
  mode,
  onSetMode,
}: {
  mode: Mode;
  onSetMode: (mode: Mode) => void;
}) {
  return <ComposerModeBar mode={mode} onSetMode={onSetMode} disabled={false} running={false} />;
}
