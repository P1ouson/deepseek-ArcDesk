import { stringStore } from "./localStorageStore";
import type { AppMode } from "./appMode";

const startupModeStore = stringStore("ARCDESK.startup.defaultMode");

export function getDefaultAppMode(): AppMode {
  const raw = startupModeStore.get();
  return raw === "write" ? "write" : "code";
}

export function setDefaultAppMode(mode: AppMode): void {
  startupModeStore.set(mode);
}
