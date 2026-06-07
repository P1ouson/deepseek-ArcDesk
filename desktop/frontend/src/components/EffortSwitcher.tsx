import { Check, ChevronsUpDown, Gauge } from "lucide-react";
import { asArray } from "../lib/array";
import { useT } from "../lib/i18n";
import type { EffortInfo } from "../lib/types";

export function EffortSwitcherMenu({
  effort,
  onPick,
}: {
  effort: EffortInfo;
  onPick: (level: string) => void;
}) {
  const levels = asArray(effort.levels);
  const current = effort.current || "auto";

  return (
    <div role="listbox">
      {levels.map((level) => (
        <button
          key={level}
          type="button"
          role="option"
          aria-selected={level === current}
          className={`modelsw__item ${level === current ? "modelsw__item--current" : ""}`}
          onClick={() => onPick(level)}
        >
          <span className="modelsw__model">{level}</span>
              {level === current && <Check size={12} className="modelsw__check" />}
        </button>
      ))}
    </div>
  );
}

export function EffortSwitcherTrigger({
  effort,
  open,
  disabled,
  onClick,
}: {
  effort: EffortInfo;
  open: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  const t = useT();
  const current = effort.current || "auto";

  return (
    <button
      type="button"
      className={`modelsw__trigger effortsw__trigger${current !== "auto" ? " effortsw__trigger--explicit" : ""}${open ? " modelsw__trigger--open" : ""}`}
      disabled={disabled}
      aria-expanded={open}
      onClick={onClick}
    >
      <Gauge size={13} className="modelsw__kind" />
      <span className="modelsw__label">{t("status.effort", { level: current })}</span>
      <ChevronsUpDown size={11} />
    </button>
  );
}

export function EffortSwitcher({
  effort,
  disabled,
  open,
  onClick,
}: {
  effort?: EffortInfo;
  disabled: boolean;
  open: boolean;
  onClick: () => void;
}) {
  if (!effort?.supported || asArray(effort.levels).length === 0) return null;

  return (
    <div className="modelsw effortsw">
      <EffortSwitcherTrigger effort={effort} open={open} disabled={disabled} onClick={onClick} />
    </div>
  );
}
