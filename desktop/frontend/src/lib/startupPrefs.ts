import type { AppMode } from "./appMode";

const DEFAULT_MODE_KEY = "ARCDESK.startup.defaultMode";

export const STARTUP_APP_MODES: AppMode[] = ["code", "write", "phone", "schedule", "plugins"];

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
