export const WORKSPACE_REF_DRAG_TYPE = "application/x-ARCDESK-workspace-ref";

export interface WorkspaceRefDragPayload {
  path: string;
  isDir?: boolean;
}

export function formatWorkspaceReference(path: string, isDir?: boolean): string {
  const clean = isDir && !path.endsWith("/") ? path + "/" : path;
  return `@${clean}`;
}

export function parseWorkspaceReference(text: string): WorkspaceRefDragPayload | null {
  const trimmed = text.trim();
  const match = /^@(\S+)$/.exec(trimmed);
  if (!match) return null;
  const path = match[1];
  if (!path) return null;
  return { path, isDir: path.endsWith("/") };
}

export function readWorkspaceReferenceDrag(dataTransfer: DataTransfer): WorkspaceRefDragPayload | null {
  if (!Array.from(dataTransfer.types).includes(WORKSPACE_REF_DRAG_TYPE)) return null;
  try {
    const payload = JSON.parse(dataTransfer.getData(WORKSPACE_REF_DRAG_TYPE)) as WorkspaceRefDragPayload;
    if (!payload.path) return null;
    return { path: payload.path, isDir: payload.isDir };
  } catch {
    return null;
  }
}

/** Parse file:// entries from a native or browser file drag payload. */
export function readDroppedFileUriPaths(dataTransfer: DataTransfer): string[] {
  const raw = dataTransfer.getData("text/uri-list") || dataTransfer.getData("text/plain");
  if (!raw) return [];
  const paths: string[] = [];
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#") || trimmed.startsWith("@")) continue;
    if (!trimmed.startsWith("file://")) continue;
    try {
      const decoded = decodeURIComponent(trimmed.replace(/^file:\/\//i, ""));
      const normalized = decoded.replace(/^\/([A-Za-z]:)/, "$1");
      paths.push(normalized);
    } catch {
      /* ignore malformed uri */
    }
  }
  return paths;
}

export function hasComposerFileDrag(dataTransfer: DataTransfer): boolean {
  const types = Array.from(dataTransfer.types);
  if (types.includes(WORKSPACE_REF_DRAG_TYPE)) return true;
  if (types.includes("Files")) return true;
  if (Array.from(dataTransfer.items).some((it) => it.kind === "file")) return true;
  return dataTransfer.files.length > 0;
}
