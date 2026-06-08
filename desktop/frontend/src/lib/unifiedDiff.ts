import type { DiffRow } from "./diff";

/** Parse a unified diff (from backend FileDiff) into render rows. */
export function unifiedDiffToRows(diff: string): DiffRow[] {
  if (!diff.trim()) return [];
  let lines = diff.replace(/\r\n/g, "\n").trimEnd().split("\n");
  if (lines.length >= 2 && lines[0]!.startsWith("--- ") && lines[1]!.startsWith("+++ ")) {
    lines = lines.slice(2);
  }
  const rows: DiffRow[] = [];
  for (const ln of lines) {
    if (!ln) continue;
    if (ln.startsWith("@@")) {
      rows.push({ type: "hunk", text: ln });
      continue;
    }
    if (ln.startsWith("+")) {
      rows.push({ type: "add", text: ln.slice(1) });
      continue;
    }
    if (ln.startsWith("-")) {
      rows.push({ type: "del", text: ln.slice(1) });
      continue;
    }
    if (ln.startsWith("\\")) continue;
    rows.push({ type: "ctx", text: ln.startsWith(" ") ? ln.slice(1) : ln });
  }
  return rows;
}
