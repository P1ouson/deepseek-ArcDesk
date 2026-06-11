import type { WireUsage } from "./types";

/** Same formulas as Reasonix / internal/usage/cache_rate.go */
export function hitRatePct(hit: number, denom: number): string | null {
  if (denom <= 0) return null;
  return ((hit / denom) * 100).toFixed(2);
}

export function stepCacheRate(u?: WireUsage): string | null {
  if (!u) return null;
  let denom = (u.cacheHitTokens ?? 0) + (u.cacheMissTokens ?? 0);
  if (denom === 0) denom = u.promptTokens ?? 0;
  return hitRatePct(u.cacheHitTokens ?? 0, denom);
}

export function sessionCacheRate(u?: WireUsage): string | null {
  if (!u) return null;
  const hit = u.sessionCacheHitTokens ?? 0;
  const denom = hit + (u.sessionCacheMissTokens ?? 0);
  return hitRatePct(hit, denom);
}
