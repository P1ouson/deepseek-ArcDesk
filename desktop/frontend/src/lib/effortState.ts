import type { EffortInfo } from "./types";

const DEEPSEEK_LEVELS = ["auto", "high", "max"] as const;

export function looksLikeDeepSeekModel(text: string): boolean {
  return text.toLowerCase().includes("deepseek");
}

/** Fallback effort UI when the backend has not hydrated yet or mis-reports capability. */
export function resolveComposerEffort(
  effort: EffortInfo | undefined,
  modelText: string,
): EffortInfo | undefined {
  if (effort?.supported && (effort.levels?.length ?? 0) > 0) {
    return effort;
  }
  if (!looksLikeDeepSeekModel(modelText)) {
    return effort;
  }
  return {
    supported: true,
    current: effort?.current?.trim() || "auto",
    default: effort?.default?.trim() || "high",
    levels: effort?.levels?.length ? effort.levels : [...DEEPSEEK_LEVELS],
  };
}
