import type { DiffRow } from "../../lib/diff";
import { highlightToHtml } from "../../lib/highlight";

const SIGN: Record<DiffRow["type"], string> = { ctx: " ", add: "+", del: "-", hunk: "@" };

export function DiffRows({
  rows,
  language,
  maxHeight,
}: {
  rows: DiffRow[];
  language?: string;
  maxHeight?: number;
}) {
  if (!rows.length) return null;
  return (
    <div className="diff hljs" style={maxHeight ? { maxHeight } : undefined}>
      {rows.map((r, idx) => (
        <div key={idx} className={`diff__row diff__row--${r.type}`}>
          <span className="diff__sign">{SIGN[r.type]}</span>
          {r.type === "hunk" ? (
            <code className="diff__text diff__text--hunk">{r.text}</code>
          ) : (
            <code
              className="diff__text"
              dangerouslySetInnerHTML={{ __html: highlightToHtml(r.text, language) }}
            />
          )}
        </div>
      ))}
    </div>
  );
}
