import type { KnowledgeEntry } from "./types";

export type KnowledgeSection = "all" | "fix" | "playbook" | "convention" | "negative" | "stale";

const NEGATIVE_RE = /\b(don't|do not|avoid|never|warning)\b|不要|别改|勿|⚠️|⚠/i;

export function entryKind(entry: KnowledgeEntry): string {
  return (entry.kind || "fix").trim().toLowerCase() || "fix";
}

export function isNegativeEntry(entry: KnowledgeEntry): boolean {
  const text = `${entry.fix}\n${entry.error}\n${entry.summary}`;
  return NEGATIVE_RE.test(text);
}

export function confidenceWeight(confidence: string): number {
  switch (confidence.trim().toLowerCase()) {
    case "user_confirmed":
      return 4;
    case "verified":
      return 3;
    case "draft":
      return 1;
    case "stale":
      return 0;
    default:
      return 2;
  }
}

export function cardTitle(entry: KnowledgeEntry): string {
  const summary = entry.summary.trim();
  if (summary) return summary.split("\n")[0];
  const sig = entry.signature.trim();
  if (sig) return sig;
  return entry.id;
}

export function cardPreview(entry: KnowledgeEntry, max = 120): string {
  const fix = entry.fix.trim();
  if (fix) return truncate(fix.split("\n")[0], max);
  return truncate(entry.summary.trim(), max);
}

export function scoreEntry(entry: KnowledgeEntry, query: string): number {
  const q = query.trim().toLowerCase();
  if (!q) {
    return confidenceWeight(entry.confidence) * 100 + Math.min(entry.hits, 99);
  }
  const tokens = q.split(/\s+/).filter(Boolean);
  const haystacks = [
    entry.id,
    entry.signature,
    entry.summary,
    entry.fix,
    entry.error,
    ...(entry.paths ?? []),
  ].map((part) => String(part ?? "").toLowerCase());

  let score = confidenceWeight(entry.confidence) * 5 + Math.min(entry.hits, 20);
  for (const token of tokens) {
    for (const hay of haystacks) {
      if (!hay.includes(token)) continue;
      score += 12;
      if (hay === entry.id.toLowerCase() || hay === entry.signature.toLowerCase()) score += 18;
    }
  }
  return score;
}

export function dedupeByFingerprint(entries: KnowledgeEntry[]): KnowledgeEntry[] {
  const seen = new Set<string>();
  const out: KnowledgeEntry[] = [];
  for (const entry of entries) {
    const key = [
      entry.signature.trim().toLowerCase(),
      entry.fix.trim().toLowerCase(),
      ...(entry.paths ?? []).map((p) => p.trim().toLowerCase()),
    ].join("\0");
    if (key && seen.has(key)) continue;
    if (key) seen.add(key);
    out.push(entry);
  }
  return out;
}

export function filterBySection(entries: KnowledgeEntry[], section: KnowledgeSection): KnowledgeEntry[] {
  switch (section) {
    case "all":
      return entries;
    case "stale":
      return entries.filter(
        (entry) =>
          entry.confidence.trim().toLowerCase() === "stale" || entry.provenanceStale === true,
      );
    case "negative":
      return entries.filter(isNegativeEntry);
    default:
      return entries.filter((entry) => entryKind(entry) === section);
  }
}

export function searchAndSort(
  entries: KnowledgeEntry[],
  query: string,
  section: KnowledgeSection,
): KnowledgeEntry[] {
  return filterBySection(entries, section)
    .filter((entry) => {
      const q = query.trim().toLowerCase();
      if (!q) return true;
      const hay = [
        entry.id,
        entry.signature,
        entry.summary,
        entry.fix,
        entry.error,
        ...(entry.paths ?? []),
      ]
        .join("\n")
        .toLowerCase();
      return q.split(/\s+/).every((token) => hay.includes(token));
    })
    .sort((a, b) => scoreEntry(b, query) - scoreEntry(a, query));
}

export function sectionCounts(entries: KnowledgeEntry[]): Record<KnowledgeSection, number> {
  return {
    all: entries.length,
    fix: filterBySection(entries, "fix").length,
    playbook: filterBySection(entries, "playbook").length,
    convention: filterBySection(entries, "convention").length,
    negative: filterBySection(entries, "negative").length,
    stale: filterBySection(entries, "stale").length,
  };
}

export function injectableCount(entries: KnowledgeEntry[]): number {
  return entries.filter(
    (entry) =>
      entry.confidence.trim().toLowerCase() !== "stale" && entry.provenanceStale !== true,
  ).length;
}

function truncate(value: string, max: number): string {
  if (value.length <= max) return value;
  return `${value.slice(0, max - 1)}…`;
}
