/** Logs bridge/controller failures without changing UI notices. */
export function logBridgeError(scope: string, err: unknown): void {
  const detail = err instanceof Error ? err.message : String(err ?? "unknown");
  console.error(`[ArcDesk] ${scope}: ${detail}`, err);
}
