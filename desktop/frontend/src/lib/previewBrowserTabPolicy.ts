/** Decide whether opening the browser preview should spawn a new tab. */
export function shouldOpenNewBrowserTab(
  browserActive: boolean,
  options?: { forceNewTab?: boolean },
): boolean {
  return Boolean(options?.forceNewTab || !browserActive);
}
