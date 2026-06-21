import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Brain, Check, ChevronsUpDown } from "lucide-react";
import { asArray } from "../lib/array";
import { app, onModelsRefreshed } from "../lib/bridge";
import { logBridgeError } from "../lib/logBridgeError";
import { useT } from "../lib/i18n";
import { modelShortLabel } from "../lib/modelLabel";
import {
  isPendingConfirmed,
  mergeSelectedCurrent,
  resolveModelDisplayLabel,
  resolvePendingRef,
} from "../lib/modelSwitcherState";
import type { ModelInfo } from "../lib/types";

/** Shared model list + display label for the composer trigger and menu. */
export function useModelSwitcher(tabId: string | undefined, fallbackLabel: string) {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [selectedRef, setSelectedRef] = useState("");
  const selectedRefRef = useRef("");
  const pendingRefRef = useRef("");
  const reloadGenRef = useRef(0);

  const syncSelectedRef = useCallback((ref: string) => {
    selectedRefRef.current = ref;
    setSelectedRef(ref);
  }, []);

  const applyModels = useCallback((incoming: ModelInfo[]) => {
    const next = asArray(incoming);
    const pending = pendingRefRef.current.trim();
    if (pending) {
      if (isPendingConfirmed(next, pending)) {
        pendingRefRef.current = "";
        syncSelectedRef(resolvePendingRef(next, pending));
        setModels(next);
        return;
      }
      syncSelectedRef(pending);
      setModels(mergeSelectedCurrent(next, pending));
      return;
    }
    const apiCurrent = next.find((m) => m.current);
    if (apiCurrent) {
      syncSelectedRef(apiCurrent.ref);
      setModels(next);
      return;
    }
    const selected = selectedRefRef.current.trim();
    setModels(selected ? mergeSelectedCurrent(next, selected) : next);
  }, [syncSelectedRef]);

  const reload = useCallback(() => {
    const gen = ++reloadGenRef.current;
    void (tabId ? app.ModelsForTab(tabId) : app.Models())
      .then((next) => {
        if (gen !== reloadGenRef.current) return;
        applyModels(next);
      })
      .catch((err) => logBridgeError("Models", err));
  }, [applyModels, tabId]);

  useEffect(() => {
    reload();
  }, [reload]);

  useEffect(() => onModelsRefreshed(reload), [reload]);

  const displayLabel = useMemo(
    () => resolveModelDisplayLabel(models, fallbackLabel, selectedRef),
    [models, fallbackLabel, selectedRef],
  );

  const markCurrent = useCallback((ref: string) => {
    const trimmed = ref.trim();
    if (!trimmed) return;
    pendingRefRef.current = trimmed;
    syncSelectedRef(trimmed);
    setModels((prev) => mergeSelectedCurrent(prev, trimmed));
  }, [syncSelectedRef]);

  const pickModel = useCallback(
    async (ref: string, switchFn: (name: string) => void | Promise<void>) => {
      const trimmed = ref.trim();
      if (!trimmed) return;
      markCurrent(trimmed);
      try {
        await switchFn(trimmed);
      } catch (err) {
        pendingRefRef.current = "";
        reloadGenRef.current += 1;
        reload();
        logBridgeError("pickModel", err);
      }
    },
    [markCurrent, reload],
  );

  return { models, displayLabel, pickModel };
}

export function ModelSwitcherMenu({
  models,
  onPick,
}: {
  models: ModelInfo[];
  onPick: (name: string) => void;
}) {
  const t = useT();

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
          onPointerDown={(event) => {
            if (event.button !== 0) return;
            event.preventDefault();
            event.stopPropagation();
            onPick(m.ref);
          }}
        >
          <span className="modelsw__model">{modelShortLabel(m.model)}</span>
          {m.current && <Check size={12} className="modelsw__check" />}
        </button>
      ))}
    </div>
  );
}

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
    >
      <Brain size={13} className="modelsw__kind" />
      <span className="modelsw__label">{label}</span>
      <ChevronsUpDown size={11} />
    </button>
  );
}
