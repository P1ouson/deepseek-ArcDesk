import { useEffect, useState } from "react";
import { ChevronDown } from "lucide-react";
import { MotionUnfold } from "./MotionUnfold";

function summarizeValues(values: string[], maxLen = 42): string {
  const text = values.join(", ");
  if (!text) return "";
  if (text.length <= maxLen) return text;
  return `${text.slice(0, maxLen - 1)}…`;
}

export function CollapsibleCommaField({
  title,
  values,
  placeholder,
  hint,
  busy,
  onCommit,
}: {
  title: string;
  values: string[];
  placeholder: string;
  hint: string;
  busy?: boolean;
  onCommit: (values: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(values.join(", "));
  const summary = summarizeValues(values);

  useEffect(() => {
    if (!open) setDraft(values.join(", "));
  }, [values, open]);

  const commit = () => {
    const next = draft
      .split(/[,\s]+/)
      .map((part) => part.trim())
      .filter(Boolean);
    onCommit(next);
    setDraft(next.join(", "));
  };

  return (
    <div className="set-rules-fold">
      <button
        type="button"
        className={`settings-agent-advanced-toggle set-rules-fold__toggle${open ? " settings-agent-advanced-toggle--open" : ""}`}
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
      >
        <ChevronDown size={13} aria-hidden="true" />
        <span className="set-rules-fold__label">{title}</span>
        {!open && summary ? <span className="set-rules-fold__summary">{summary}</span> : null}
      </button>
      <MotionUnfold open={open}>
        <div className="set-rules-fold__body set-rules-fold__body--field">
            <input
              className="mem-input settings-block__input"
              value={draft}
              disabled={busy}
              placeholder={placeholder}
              spellCheck={false}
              onChange={(e) => setDraft(e.target.value)}
              onBlur={commit}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  commit();
                  (e.currentTarget as HTMLInputElement).blur();
                }
              }}
            />
            <p className="settings-block__note settings-block__note--inline">{hint}</p>
        </div>
      </MotionUnfold>
    </div>
  );
}

function parsePreviewPorts(values: string[]): number[] {
  const seen = new Set<number>();
  const out: number[] = [];
  for (const raw of values) {
    const port = Number.parseInt(raw, 10);
    if (!Number.isFinite(port) || port < 1 || port > 65535 || seen.has(port)) continue;
    seen.add(port);
    out.push(port);
  }
  return out.sort((a, b) => a - b);
}

export function CollapsiblePreviewPortsField({
  title,
  ports,
  placeholder,
  hint,
  busy,
  onCommit,
}: {
  title: string;
  ports: number[];
  placeholder: string;
  hint: string;
  busy?: boolean;
  onCommit: (ports: number[]) => void;
}) {
  const stringValues = ports.map(String);
  return (
    <CollapsibleCommaField
      title={title}
      values={stringValues}
      placeholder={placeholder}
      hint={hint}
      busy={busy}
      onCommit={(next) => onCommit(parsePreviewPorts(next))}
    />
  );
}
