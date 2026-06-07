import type { ComponentProps, ReactNode } from "react";
import { AlertTriangle, List, Zap } from "lucide-react";
import { Composer } from "./Composer";
import { useT } from "../lib/i18n";
import type { Mode } from "../lib/types";

export type FloatingComposerProps = ComponentProps<typeof Composer>;

const MODE_OPTIONS: Array<{ id: Mode; label: string; icon: ReactNode }> = [
  { id: "normal", label: "auto", icon: <Zap size={13} /> },
  { id: "plan", label: "plan", icon: <List size={13} /> },
  { id: "yolo", label: "yolo", icon: <AlertTriangle size={13} /> },
];

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
    <div className="composer-modebar" role="toolbar" aria-label={t("composer.modeTitle")}>
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

export function FloatingComposer(props: FloatingComposerProps) {
  const planActive = props.mode === "plan";
  return (
    <div className={`composer-shell${planActive ? " composer-shell--plan" : ""}`}>
      <Composer {...props} hideModeBar />
    </div>
  );
}

/** @deprecated Use ComposerModeBar via FloatingComposer */
export function ComposerModeToggle({
  mode,
  onSetMode,
}: {
  mode: Mode;
  onSetMode: (mode: Mode) => void;
}) {
  return <ComposerModeBar mode={mode} onSetMode={onSetMode} disabled={false} running={false} />;
}

export function SlashSuggestionPills({
  visible,
  onPick,
}: {
  visible: boolean;
  onPick: (cmd: string) => void;
}) {
  if (!visible) return null;
  const cmds = ["/plan", "/review", "/btw", "/goal", "/sdd"];
  return (
    <div className="composer-shell__cmdrow">
      {cmds.map((cmd) => (
        <button key={cmd} type="button" className="composer-shell__slash-pill" onClick={() => onPick(`${cmd} `)}>
          {cmd}
        </button>
      ))}
    </div>
  );
}
