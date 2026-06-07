const STORAGE_KEY = "reasonix.workspaceRecents.v1";
const MAX_STORED = 12;

function readRecents(): string[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((path): path is string => typeof path === "string" && path.trim() !== "");
  } catch {
    return [];
  }
}

function writeRecents(paths: string[]) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(paths.slice(0, MAX_STORED)));
  } catch {
    /* ignore quota errors */
  }
}

export function getRecentWorkspacePaths(): string[] {
  return readRecents();
}

export function recordRecentWorkspace(path: string) {
  const normalized = path.trim();
  if (!normalized) return;
  const next = [normalized, ...readRecents().filter((item) => item !== normalized)];
  writeRecents(next);
}

export function removeRecentWorkspace(path: string) {
  writeRecents(readRecents().filter((item) => item !== path));
}
