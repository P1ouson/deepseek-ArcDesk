export function prefersReducedMotion(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") return false;
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

export function motionDurationMs(baseMs: number): number {
  return prefersReducedMotion() ? 0 : baseMs;
}
