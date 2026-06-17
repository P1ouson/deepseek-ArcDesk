/** Normalize filesystem paths for write-mode browsing (forward slashes, no trailing slash). */
export function normalizeWritePath(path: string): string {
  return path.replace(/\\/g, "/").replace(/\/+$/, "");
}

function pathKey(path: string): string {
  const norm = normalizeWritePath(path);
  if (/^[a-zA-Z]:/.test(norm)) {
    return norm.toLowerCase();
  }
  return norm;
}

export function pathsEqual(a: string, b: string): boolean {
  return pathKey(a) === pathKey(b);
}

export function isPathUnderRoot(path: string, root: string): boolean {
  const p = pathKey(path);
  const r = pathKey(root);
  return p === r || p.startsWith(`${r}/`);
}

export function isDirectChild(entryPath: string, parentPath: string): boolean {
  const parent = pathKey(parentPath);
  const entry = pathKey(entryPath);
  if (entry === parent || !entry.startsWith(`${parent}/`)) return false;
  const rel = entry.slice(parent.length + 1);
  return rel.length > 0 && !rel.includes("/");
}

export function parentWritePath(path: string): string {
  const normalized = normalizeWritePath(path);
  const idx = normalized.lastIndexOf("/");
  if (idx <= 0) return normalized;
  return normalized.slice(0, idx);
}
