/** Composer send gate — mirrors App.tsx workbench Composer `disabled=`. */
export function isComposerSendDisabled(
  meta: { ready?: boolean; startupErr?: string } | undefined,
  approval: unknown,
  ask: unknown,
  runtimeReady = true,
): boolean {
  return meta?.ready === false || !!meta?.startupErr || approval != null || ask != null || !runtimeReady;
}
