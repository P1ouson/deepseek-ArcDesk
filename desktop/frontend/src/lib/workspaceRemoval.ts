import { app } from "./bridge";
import {
  clearStoredCodeWorkspaceRoot,
  getStoredCodeWorkspaceRoot,
  sameWorkspaceRoot,
  setStoredComposerNoWorkspace,
} from "./composerWorkspace";

export function isPathUnderWorkspace(filePath: string, workspaceRoot: string): boolean {
  const file = filePath.trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
  const root = workspaceRoot.trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
  if (!file || !root) return false;
  return file === root || file.startsWith(`${root}/`);
}

/** Drop composer/dock binding when a sidebar project is removed. */
export async function syncComposerAfterWorkspaceRemoved(removedPath: string): Promise<{
  noProjectsLeft: boolean;
  detached: boolean;
}> {
  const workspaces = await app.ListWorkspaces();
  const noProjectsLeft = workspaces.length === 0;
  const detached =
    noProjectsLeft || sameWorkspaceRoot(removedPath, getStoredCodeWorkspaceRoot());
  if (detached) {
    clearStoredCodeWorkspaceRoot();
    setStoredComposerNoWorkspace(true);
  }
  return { noProjectsLeft, detached };
}
