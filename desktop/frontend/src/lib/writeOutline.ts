export interface WriteOutlineItem {
  level: number;
  text: string;
  line: number;
}

export function extractMarkdownOutline(content: string): WriteOutlineItem[] {
  const lines = content.split(/\r?\n/);
  const items: WriteOutlineItem[] = [];
  for (let i = 0; i < lines.length; i++) {
    const match = /^(#{1,6})\s+(.+?)\s*$/.exec(lines[i] ?? "");
    if (!match) continue;
    items.push({
      level: match[1]!.length,
      text: match[2]!.replace(/\s+#+\s*$/, "").trim(),
      line: i + 1,
    });
  }
  return items;
}
