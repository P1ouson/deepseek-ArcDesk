import { useCallback, useEffect, useState } from "react";
import { Brain, Check, ChevronsUpDown } from "lucide-react";
import { asArray } from "../lib/array";
import { app, onModelsRefreshed } from "../lib/bridge";
import { logBridgeError } from "../lib/logBridgeError";
import { useT } from "../lib/i18n";
import { modelShortLabel } from "../lib/modelLabel";
import type { ModelInfo } from "../lib/types";
export function ModelSwitcherMenu({
  tabId,
  onPick,
}: {
  tabId?: string;
  onPick: (name: string) => void;
}) {
  const t = useT();
  const [models, setModels] = useState<ModelInfo[]>([]);

  const reload = useCallback(() => {
    void (tabId ? app.ModelsForTab(tabId) : app.Models())
      .then((next) => setModels(asArray(next)))
      .catch((err) => logBridgeError("Models", err));
  }, [tabId]);

  useEffect(() => {
    reload();
  }, [reload]);

  useEffect(() => onModelsRefreshed(reload), [reload]);

  return (
    <div className="modelsw__list" role="listbox">
      {models.length === 0 && <div className="modelsw__empty">{t("status.noModels")}</div>}
      {models.map((m) => (
        <button
          key={m.ref}
          type="button"
          role="option"
          aria-selected={m.current}
          className={`modelsw__item ${m.current ? "modelsw__item--current" : ""}`}
          title={m.model}
          onClick={() => onPick(m.ref)}
        >
          <span className="modelsw__model">{modelShortLabel(m.model)}</span>
          {m.current && <Check size={12} className="modelsw__check" />}
        </button>
      ))}
    </div>
  );}

export function ModelSwitcherTrigger({
  label,
  title,
  open,
  disabled,
  onClick,
}: {
  label: string;
  title?: string;
  open: boolean;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className={`modelsw__trigger${open ? " modelsw__trigger--open" : ""}`}
      aria-expanded={open}
      disabled={disabled}
      title={title || label}
      onClick={onClick}
    >      <Brain size={13} className="modelsw__kind" />
      <span className="modelsw__label">{label}</span>
      <ChevronsUpDown size={11} />
    </button>
  );
}
