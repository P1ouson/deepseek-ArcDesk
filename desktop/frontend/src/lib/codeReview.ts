export type ReviewMode = "standard" | "security";
export type ReviewScope = "all" | "session" | "git" | "both";

export type ReviewFindingSeverity =
  | "blocking"
  | "should-fix"
  | "nit"
  | "critical"
  | "high"
  | "medium"
  | "info";

export interface ReviewFinding {
  severity: ReviewFindingSeverity;
  text: string;
  file?: string;
  line?: number;
}

export type ReviewVerdictTone = "ok" | "warn" | "block" | "unknown";

export interface ParsedReview {
  verdict?: string;
  verdictTone: ReviewVerdictTone;
  findings: ReviewFinding[];
  raw: string;
}

const FILE_LINE_RE = /([^\s:]+\.[A-Za-z0-9]+):(\d+)/;

function severityFromHeading(line: string): ReviewFindingSeverity | null {
  const lower = line.toLowerCase();
  if (/blocking|do-not-ship|do not ship|critical/.test(lower)) return "blocking";
  if (/\bcritical\b/.test(lower)) return "critical";
  if (/\bhigh\b/.test(lower)) return "high";
  if (/\bmedium\b/.test(lower)) return "medium";
  if (/should-fix|should fix|minor|warning/.test(lower)) return "should-fix";
  if (/\bnits?\b/.test(lower)) return "nit";
  return null;
}

function verdictToneFromText(text: string): ReviewVerdictTone {
  const lower = text.toLowerCase();
  if (/blocking|do-not-ship|do not ship|cannot ship|不能合|阻塞|阻断/.test(lower)) return "block";
  if (/ship as-is|no issues|looks clean|no security issues|可以合|没问题|干净/.test(lower)) return "ok";
  if (/minor|nit|should-fix|small concern|小问|修完/.test(lower)) return "warn";
  return "unknown";
}

function parseFileLine(text: string): { file?: string; line?: number } {
  const match = FILE_LINE_RE.exec(text);
  if (!match) return {};
  return { file: match[1], line: Number.parseInt(match[2]!, 10) };
}

export function parseReviewMarkdown(raw: string): ParsedReview {
  const lines = raw.split(/\r?\n/);
  let verdict: string | undefined;
  let currentSeverity: ReviewFindingSeverity = "info";
  const findings: ReviewFinding[] = [];

  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) continue;

    if (!verdict && /^(-|\*)?\s*(verdict|结论|审查结论)/i.test(trimmed)) {
      verdict = trimmed.replace(/^(-|\*)?\s*(verdict|结论|审查结论)\s*:?\s*/i, "");
      continue;
    }

    if (!verdict && /^(ship|blocking|minor|no security|no issues|可以|不能|建议)/i.test(trimmed) && trimmed.length < 160) {
      verdict = trimmed.replace(/^[-*]\s*/, "");
    }

    const headingSeverity = severityFromHeading(trimmed);
    if (headingSeverity && trimmed.length < 80 && (trimmed.endsWith(":") || /^#{1,3}\s/.test(trimmed))) {
      currentSeverity = headingSeverity;
      continue;
    }

    const bullet = trimmed.match(/^[-*•]\s+(.+)$/);
    if (bullet) {
      const text = bullet[1]!;
      const loc = parseFileLine(text);
      findings.push({ severity: currentSeverity, text, ...loc });
      continue;
    }

    const numbered = trimmed.match(/^\d+[.)]\s+(.+)$/);
    if (numbered) {
      const text = numbered[1]!;
      const loc = parseFileLine(text);
      findings.push({ severity: currentSeverity, text, ...loc });
    }
  }

  const verdictTone = verdictToneFromText(`${verdict ?? ""}\n${raw.slice(0, 400)}`);
  return { verdict, verdictTone, findings, raw };
}
