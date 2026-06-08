import { NO_WORKSPACE_VALUE } from "./composerWorkspace";

const ROOT_KEY = "ARCDESK.writeWorkspaceRoot.v1";

export { NO_WORKSPACE_VALUE };

export function getStoredWriteWorkspaceRoot(): string {
  if (typeof window === "undefined") return "";
  try {
    const raw = window.localStorage.getItem(ROOT_KEY);
    if (typeof raw !== "string") return "";
    const trimmed = raw.trim();
    return trimmed === NO_WORKSPACE_VALUE ? NO_WORKSPACE_VALUE : trimmed;
  } catch {
    return "";
  }
}

/** Initial write workspace from cache only — never auto-pick a default folder. */
export function getInitialWriteWorkspaceRoot(): string {
  const stored = getStoredWriteWorkspaceRoot();
  if (isNoWriteWorkspace(stored)) return NO_WORKSPACE_VALUE;
  return stored;
}

export function setStoredWriteWorkspaceRoot(path: string) {
  if (typeof window === "undefined") return;
  const normalized = path.trim();
  if (!normalized) return;
  try {
    window.localStorage.setItem(ROOT_KEY, normalized);
  } catch {
    /* ignore quota errors */
  }
}

export function isNoWriteWorkspace(path: string | undefined | null): boolean {
  return (path ?? "").trim() === NO_WORKSPACE_VALUE;
}

export function isUsableWriteWorkspaceRoot(path: string | undefined | null): boolean {
  const normalized = (path ?? "").trim();
  return normalized !== "" && normalized !== "." && normalized !== NO_WORKSPACE_VALUE;
}
