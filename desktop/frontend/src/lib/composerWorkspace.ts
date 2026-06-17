import { stringStore } from "./localStorageStore";
import type { TabMeta } from "./types";

export const NO_WORKSPACE_VALUE = "__no_workspace__";

const codeRootStore = stringStore("ARCDESK.codeWorkspaceRoot.v1");
const composerNoWorkspaceStore = stringStore("ARCDESK.composerNoWorkspace.v1");

export function getStoredCodeWorkspaceRoot(): string {
  return codeRootStore.get();
}

export function setStoredCodeWorkspaceRoot(path: string): void {
  const normalized = path.trim();
  if (!isUsableCodeWorkspaceRoot(normalized)) {
    codeRootStore.remove();
    return;
  }
  codeRootStore.set(normalized);
}

export function clearStoredCodeWorkspaceRoot(): void {
  codeRootStore.remove();
}

export function isUsableCodeWorkspaceRoot(path: string | undefined | null): boolean {
  const normalized = (path ?? "").trim();
  return normalized !== "" && normalized !== "." && !isNoWorkspaceRoot(normalized);
}

const BROAD_PERSONAL_SUFFIXES = [
  "/desktop",
  "/documents",
  "/downloads",
  "/documents/",
  "/downloads/",
  "/desktop/",
];

/** True for user home shells (Desktop/Documents/Downloads) — too broad to index or explore as a code project. */
export function isBroadPersonalRoot(path: string | undefined | null): boolean {
  const normalized = (path ?? "").trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
  if (!normalized) return false;
  for (const suffix of BROAD_PERSONAL_SUFFIXES) {
    const s = suffix.replace(/\/+$/, "");
    if (normalized === s || normalized.endsWith(s)) return true;
  }
  return false;
}

/** Code workspace suitable for agent tool confinement and repo-map indexing. */
export function isProjectLikeCodeWorkspaceRoot(path: string | undefined | null): boolean {
  return isUsableCodeWorkspaceRoot(path) && !isBroadPersonalRoot(path);
}

export function getStoredComposerNoWorkspace(): boolean {
  if (typeof window === "undefined") return true;
  try {
    const explicit = window.localStorage.getItem("ARCDESK.composerNoWorkspace.v1");
    if (explicit === "1") return true;
    if (getStoredCodeWorkspaceRoot()) return false;
    if (explicit === "0") return false;
    return true;
  } catch {
    return true;
  }
}

export function setStoredComposerNoWorkspace(enabled: boolean) {
  if (enabled) composerNoWorkspaceStore.set("1");
  else composerNoWorkspaceStore.remove();
}

export function isNoWorkspaceRoot(path: string | undefined | null): boolean {
  const normalized = (path ?? "").trim();
  return normalized === "" || normalized === NO_WORKSPACE_VALUE;
}

/** Case-insensitive path compare for Windows drive letters and separators. */
export function sameWorkspaceRoot(a: string | undefined | null, b: string | undefined | null): boolean {
  const left = (a ?? "").trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
  const right = (b ?? "").trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
  return left !== "" && left === right;
}

/** Workspace bound to the active project tab — never a stale localStorage fallback. */
export function activeCodeWorkspaceRoot(tab: TabMeta | undefined): string | undefined {
  if (!tab || tab.scope !== "project") return undefined;
  const root = tab.workspaceRoot?.trim();
  return isUsableCodeWorkspaceRoot(root) ? root : undefined;
}

export type CodeWorkspaceReconcile =
  | { kind: "noop" }
  | { kind: "syncToActive"; path: string }
  | { kind: "clearOrphan" };

/** Align persisted composer workspace with the restored session on startup. */
export function planCodeWorkspaceReconcile(
  activeTab: TabMeta | undefined,
  openTabs: TabMeta[],
  storedRoot: string,
): CodeWorkspaceReconcile {
  const activeRoot = activeCodeWorkspaceRoot(activeTab);
  if (activeRoot) {
    if (!sameWorkspaceRoot(storedRoot, activeRoot)) {
      return { kind: "syncToActive", path: activeRoot };
    }
    return { kind: "noop" };
  }
  if (!isUsableCodeWorkspaceRoot(storedRoot)) {
    return { kind: "noop" };
  }
  const storedStillOpen = openTabs.some(
    (tab) => tab.scope === "project" && sameWorkspaceRoot(tab.workspaceRoot, storedRoot),
  );
  if (!storedStillOpen) {
    return { kind: "clearOrphan" };
  }
  return { kind: "noop" };
}
