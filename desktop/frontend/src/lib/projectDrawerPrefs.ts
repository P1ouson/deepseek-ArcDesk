const CANONICAL_KEY = "ARCDESK.studio.projectDrawerOpen";
const LEGACY_KEY = "arcdesk.studio.projectDrawerOpen";
const LEGACY_SIDEBAR_COLLAPSED_KEY = "arcdesk.sidebar.collapsed";

export function loadProjectDrawerOpen(): boolean {
  if (typeof window === "undefined") return true;
  try {
    const stored = window.localStorage.getItem(CANONICAL_KEY) ?? window.localStorage.getItem(LEGACY_KEY);
    if (stored === "1") return true;
    if (stored === "0") return false;
    const legacyCollapsed = window.localStorage.getItem(LEGACY_SIDEBAR_COLLAPSED_KEY);
    if (legacyCollapsed === "1") return false;
    if (legacyCollapsed === "0") return true;
    return true;
  } catch {
    return true;
  }
}

export function saveProjectDrawerOpen(open: boolean): void {
  if (typeof window === "undefined") return;
  try {
    const value = open ? "1" : "0";
    window.localStorage.setItem(CANONICAL_KEY, value);
    window.localStorage.setItem(LEGACY_KEY, value);
    window.localStorage.setItem(LEGACY_SIDEBAR_COLLAPSED_KEY, open ? "0" : "1");
  } catch {
    /* ignore storage failures */
  }
}
