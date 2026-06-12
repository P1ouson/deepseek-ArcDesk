import { stringStore } from "./localStorageStore";

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
