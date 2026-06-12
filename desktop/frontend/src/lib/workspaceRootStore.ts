import {
  clearStoredCodeWorkspaceRoot,
  getStoredCodeWorkspaceRoot,
  getStoredComposerNoWorkspace,
  isNoWorkspaceRoot,
  isUsableCodeWorkspaceRoot,
  sameWorkspaceRoot,
  setStoredCodeWorkspaceRoot,
  setStoredComposerNoWorkspace,
  NO_WORKSPACE_VALUE,
} from "./composerWorkspace";
import {
  getInitialWriteWorkspaceRoot,
  getStoredWriteWorkspaceRoot,
  isNoWriteWorkspace,
  isUsableWriteWorkspaceRoot,
  setStoredWriteWorkspaceRoot,
} from "./writeWorkspace";
import { getDefaultAppMode, setDefaultAppMode } from "./startupPrefs";

export {
  NO_WORKSPACE_VALUE,
  clearStoredCodeWorkspaceRoot,
  getStoredCodeWorkspaceRoot,
  getStoredComposerNoWorkspace,
  isNoWorkspaceRoot,
  isUsableCodeWorkspaceRoot,
  sameWorkspaceRoot,
  setStoredCodeWorkspaceRoot,
  setStoredComposerNoWorkspace,
  getInitialWriteWorkspaceRoot,
  getStoredWriteWorkspaceRoot,
  isNoWriteWorkspace,
  isUsableWriteWorkspaceRoot,
  setStoredWriteWorkspaceRoot,
  getDefaultAppMode,
  setDefaultAppMode,
};

/** Code-mode workspace root, or no-workspace sentinel. */
export function getCodeWorkspaceRootOrNone(): string {
  if (getStoredComposerNoWorkspace()) return NO_WORKSPACE_VALUE;
  return getStoredCodeWorkspaceRoot();
}
