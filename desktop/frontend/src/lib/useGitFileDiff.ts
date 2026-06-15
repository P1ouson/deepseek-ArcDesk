import { useEffect, useState } from "react";
import { app } from "./bridge";
import { shellQuote } from "./shellQuote";

export type GitFileDiffPreview = {
  added: number;
  removed: number;
  lines: { kind: "add" | "del" | "ctx"; text: string; no: number }[];
  loading: boolean;
  error?: string;
};

function parseDiffStats(text: string): { added: number; removed: number } {
  let added = 0;
  let removed = 0;
  for (const line of text.split("\n")) {
    if (line.startsWith("+") && !line.startsWith("+++")) added += 1;
    if (line.startsWith("-") && !line.startsWith("---")) removed += 1;
  }
  return { added, removed };
}

function toPreviewLines(raw: string, max = 48): GitFileDiffPreview["lines"] {
  const out: GitFileDiffPreview["lines"] = [];
  let no = 1;
  for (const line of raw.split("\n")) {
    if (out.length >= max) break;
    if (line.startsWith("+++") || line.startsWith("---") || line.startsWith("diff ")) continue;
    if (line.startsWith("+")) {
      out.push({ kind: "add", text: line.slice(1), no: no++ });
    } else if (line.startsWith("-")) {
      out.push({ kind: "del", text: line.slice(1), no: no++ });
    } else if (line.startsWith("@@")) {
      out.push({ kind: "ctx", text: line, no: no++ });
    } else {
      out.push({ kind: "ctx", text: line.replace(/^\s*/, ""), no: no++ });
    }
  }
  return out;
}

export function useGitFileDiff(path: string, gitStatus: string | undefined, enabled: boolean) {
  const [preview, setPreview] = useState<GitFileDiffPreview>({
    added: 0,
    removed: 0,
    lines: [],
    loading: false,
  });

  useEffect(() => {
    if (!enabled || !path) return;
    let cancelled = false;
    setPreview((prev) => ({ ...prev, loading: true, error: undefined }));

    void (async () => {
      try {
        const untracked = gitStatus?.includes("?") || gitStatus?.includes("A") || gitStatus?.includes("New");
        if (untracked) {
          const file = await app.ReadFile(path);
          const content = file.body ?? "";
          const rawLines = content.split("\n").slice(0, 48);
          const lines = rawLines.map((text: string, i: number) => ({ kind: "add" as const, text, no: i + 1 }));
          if (!cancelled) {
            setPreview({ added: rawLines.length, removed: 0, lines, loading: false });
          }
          return;
        }
        const result = await app.RunShellQuiet(`git diff -- ${shellQuote(path)}`);
        const text = `${result.output}\n${result.err ?? ""}`.trim();
        const stats = parseDiffStats(text);
        if (!cancelled) {
          setPreview({
            ...stats,
            lines: toPreviewLines(text),
            loading: false,
            error: result.err && !result.output ? result.err : undefined,
          });
        }
      } catch (e) {
        if (!cancelled) {
          setPreview({ added: 0, removed: 0, lines: [], loading: false, error: String(e) });
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [enabled, path, gitStatus]);

  return preview;
}
