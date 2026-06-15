export function isWordDocumentPath(path?: string | null): boolean {
  if (!path) return false;
  const lower = path.toLowerCase();
  return lower.endsWith(".docx") || (lower.endsWith(".doc") && !lower.endsWith(".docx"));
}
