/** First non-empty line, truncated for compact UI labels. */
export function truncateOneLine(text: string, max = 72): string {
  const line = text.trim().split("\n").find((entry) => entry.trim())?.trim() ?? text.trim();
  if (line.length <= max) return line;
  return `${line.slice(0, max - 1)}…`;
}
