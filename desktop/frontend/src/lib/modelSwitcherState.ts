import type { ModelInfo } from "./types";
import { modelLabelFromRef, modelShortLabel } from "./modelLabel";

/** Resolve the model SKU (e.g. deepseek-v4-pro) for a provider/model ref. */
export function targetModelId(models: ModelInfo[], ref: string): string {
  const trimmed = ref.trim();
  if (!trimmed) return "";
  return models.find((m) => m.ref === trimmed)?.model ?? modelLabelFromRef(trimmed);
}

/** Apply optimistic/current flags for the chosen model SKU, not just the raw ref. */
export function mergeSelectedCurrent(models: ModelInfo[], selectedRef: string): ModelInfo[] {
  const targetId = targetModelId(models, selectedRef);
  if (!targetId) return models;
  return models.map((m) => ({ ...m, current: m.model === targetId }));
}

export function isPendingConfirmed(models: ModelInfo[], pendingRef: string): boolean {
  const targetId = targetModelId(models, pendingRef);
  if (!targetId) return false;
  return models.some((m) => m.current && m.model === targetId);
}

export function resolvePendingRef(models: ModelInfo[], pendingRef: string): string {
  const trimmed = pendingRef.trim();
  if (!trimmed) return "";
  const targetId = targetModelId(models, trimmed);
  return (
    models.find((m) => m.current && m.model === targetId)?.ref
    ?? models.find((m) => m.ref === trimmed)?.ref
    ?? trimmed
  );
}

export function resolveModelDisplayLabel(
  models: ModelInfo[],
  fallbackLabel: string,
  selectedRef: string,
): string {
  const trimmed = selectedRef.trim();
  if (trimmed) {
    const sku = models.find((m) => m.ref === trimmed)?.model ?? targetModelId(models, trimmed);
    if (sku) return modelShortLabel(sku);
  }

  const fromMeta = modelLabelFromRef(fallbackLabel);
  if (fromMeta) return fromMeta;

  const trimmedMeta = fallbackLabel.trim();
  if (trimmedMeta) return modelShortLabel(trimmedMeta);

  const current = models.find((m) => m.current);
  if (current) return modelShortLabel(current.model);

  return "";
}
