import type { AppMode } from "./appMode";
import type { RightDockTab } from "../components/Topbar";

const DEFAULT_MODE_KEY = "ARCDESK.startup.defaultMode";
const PROJECT_DRAWER_KEY = "ARCDESK.studio.projectDrawerOpen";
const DEFAULT_DOCK_TAB_KEY = "ARCDESK.startup.defaultDockTab";

export const STARTUP_APP_MODES: AppMode[] = ["code", "write", "phone", "schedule", "plugins"];

export const STARTUP_DOCK_TABS: RightDockTab[] = ["changes", "files", "todo", "git", "context"];

export function isStartupAppMode(value: unknown): value is AppMode {
  return typeof value === "string" && (STARTUP_APP_MODES as readonly string[]).includes(value);
}

export function getDefaultAppMode(): AppMode {
  if (typeof window === "undefined") return "code";
  try {
    const stored = window.localStorage.getItem(DEFAULT_MODE_KEY);
    return isStartupAppMode(stored) ? stored : "code";
  } catch {
    return "code";
  }
}

export function setDefaultAppMode(mode: AppMode): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(DEFAULT_MODE_KEY, mode);
  } catch {
    /* ignore */
  }
}

export function getDefaultProjectDrawerOpen(): boolean {
  if (typeof window === "undefined") return true;
  try {
    const stored = window.localStorage.getItem(PROJECT_DRAWER_KEY);
    if (stored === "1") return true;
    if (stored === "0") return false;
    return true;
  } catch {
    return true;
  }
}

export function setDefaultProjectDrawerOpen(open: boolean): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(PROJECT_DRAWER_KEY, open ? "1" : "0");
  } catch {
    /* ignore */
  }
}

export function getDefaultDockTab(): RightDockTab {
  if (typeof window === "undefined") return "changes";
  try {
    const stored = window.localStorage.getItem(DEFAULT_DOCK_TAB_KEY);
    if (stored && (STARTUP_DOCK_TABS as readonly string[]).includes(stored)) {
      return stored as RightDockTab;
    }
  } catch {
    /* ignore */
  }
  return "changes";
}

export function setDefaultDockTab(tab: RightDockTab): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(DEFAULT_DOCK_TAB_KEY, tab);
  } catch {
    /* ignore */
  }
}
