export type WritePreviewKind = "word" | "markdown" | "plain";

export function isWordDocumentPath(path?: string | null): boolean {
  if (!path) return false;
  const lower = path.toLowerCase();
  return lower.endsWith(".docx") || (lower.endsWith(".doc") && !lower.endsWith(".docx"));
}

export function isMarkdownDocumentPath(path?: string | null): boolean {
  if (!path) return false;
  const lower = path.toLowerCase();
  return lower.endsWith(".md") || lower.endsWith(".markdown") || lower.endsWith(".mdx");
}

export function getWritePreviewKind(path?: string | null): WritePreviewKind {
  if (isWordDocumentPath(path)) return "word";
  if (isMarkdownDocumentPath(path)) return "markdown";
  return "plain";
}
