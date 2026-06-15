import { NO_WORKSPACE_VALUE } from "./composerWorkspace";
import { stringStore } from "./localStorageStore";

const writeRootStore = stringStore("ARCDESK.writeWorkspaceRoot.v1");

export { NO_WORKSPACE_VALUE };

export function getStoredWriteWorkspaceRoot(): string {
  const stored = writeRootStore.get();
  return stored === NO_WORKSPACE_VALUE ? NO_WORKSPACE_VALUE : stored;
}

/** Initial write workspace from cache only — never auto-pick a default folder. */
export function getInitialWriteWorkspaceRoot(): string {
  const stored = getStoredWriteWorkspaceRoot();
  if (isNoWriteWorkspace(stored)) return NO_WORKSPACE_VALUE;
  return stored;
}

export function setStoredWriteWorkspaceRoot(path: string) {
  const normalized = path.trim();
  if (!normalized) return;
  writeRootStore.set(normalized);
}

export function isNoWriteWorkspace(path: string | undefined | null): boolean {
  return (path ?? "").trim() === NO_WORKSPACE_VALUE;
}

export function isUsableWriteWorkspaceRoot(path: string | undefined | null): boolean {
  const normalized = (path ?? "").trim();
  return normalized !== "" && normalized !== "." && !isNoWriteWorkspace(normalized);
}

/** Parent directory of a document path for binding the write workspace folder. */
export function writeDocumentParentDir(filePath: string): string {
  const normalized = filePath.replace(/\\/g, "/").replace(/\/+$/, "");
  const slash = normalized.lastIndexOf("/");
  if (slash <= 0) return normalized;
  return normalized.slice(0, slash);
}
