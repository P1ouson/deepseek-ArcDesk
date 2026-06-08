export const NO_WORKSPACE_VALUE = "__no_workspace__";

const CODE_KEY = "ARCDESK.composerNoWorkspace.v1";
const CODE_ROOT_KEY = "ARCDESK.codeWorkspaceRoot.v1";

export function getStoredCodeWorkspaceRoot(): string {
  if (typeof window === "undefined") return "";
  try {
    const raw = window.localStorage.getItem(CODE_ROOT_KEY);
    return typeof raw === "string" ? raw.trim() : "";
  } catch {
    return "";
  }
}

export function setStoredCodeWorkspaceRoot(path: string): void {
  if (typeof window === "undefined") return;
  const normalized = path.trim();
  try {
    if (!isUsableCodeWorkspaceRoot(normalized)) {
      window.localStorage.removeItem(CODE_ROOT_KEY);
      return;
    }
    window.localStorage.setItem(CODE_ROOT_KEY, normalized);
  } catch {
    /* ignore */
  }
}

export function clearStoredCodeWorkspaceRoot(): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.removeItem(CODE_ROOT_KEY);
  } catch {
    /* ignore */
  }
}

export function isUsableCodeWorkspaceRoot(path: string | undefined | null): boolean {
  const normalized = (path ?? "").trim();
  return normalized !== "" && normalized !== "." && !isNoWorkspaceRoot(normalized);
}

export function getStoredComposerNoWorkspace(): boolean {
  if (typeof window === "undefined") return true;
  try {
    const explicit = window.localStorage.getItem(CODE_KEY);
    if (explicit === "1") return true;
    if (getStoredCodeWorkspaceRoot()) return false;
    if (explicit === "0") return false;
    return true;
  } catch {
    return true;
  }
}

export function setStoredComposerNoWorkspace(enabled: boolean) {
  if (typeof window === "undefined") return;
  try {
    if (enabled) window.localStorage.setItem(CODE_KEY, "1");
    else window.localStorage.removeItem(CODE_KEY);
  } catch {
    /* ignore */
  }
}

export function isNoWorkspaceRoot(path: string | undefined | null): boolean {
  const normalized = (path ?? "").trim();
  return normalized === "" || normalized === NO_WORKSPACE_VALUE;
}
