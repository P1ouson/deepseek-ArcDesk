/** DeepSeek default: cache-hit prompt tokens ≈ 2% of input price (see provider pricing). */
export const DEFAULT_CACHE_HIT_INPUT_RATIO = 0.02;

/** Estimated prompt-token savings vs billing every prompt token at input price. */
export function estimatePromptCacheSavingsPct(
  hitTokens: number,
  missTokens: number,
  hitInputRatio = DEFAULT_CACHE_HIT_INPUT_RATIO,
): number | null {
  const denom = hitTokens + missTokens;
  if (denom <= 0 || hitInputRatio <= 0 || hitInputRatio >= 1) {
    return null;
  }
  return Math.round((hitTokens / denom) * (1 - hitInputRatio) * 1000) / 10;
}
