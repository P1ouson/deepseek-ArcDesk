export function isPreviewablePagePath(path: string): boolean {
  return /\.(html?|xhtml|svg)$/i.test(path.trim());
}

export function basename(path: string): string {
  const clean = path.replace(/\\/g, "/").replace(/\/+$/, "");
  const parts = clean.split("/").filter(Boolean);
  return parts[parts.length - 1] ?? path;
}
