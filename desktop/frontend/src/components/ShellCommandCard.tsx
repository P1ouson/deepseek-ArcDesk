import { memo, useEffect, useMemo, useState } from "react";
import { ChevronRight, Loader2, Terminal } from "lucide-react";
import { MotionUnfold } from "./MotionUnfold";
import { CodeViewer } from "./CodeViewer";
import { parseToolArgs, toolArgString } from "../lib/parseToolArgs";
import type { ToolItem } from "../lib/actionStream";
import { useShellExpand } from "../lib/shellExpand";
import { useT } from "../lib/i18n";

const TAG_RE =
  /\b(?:cd|npm|npx|yarn|pnpm|bun|go|git|python|py|node|cargo|make|docker|wails|vitest|tsc|eslint|powershell|pwsh|bash)\b/gi;

function shellCommand(item: ToolItem): string {
  return toolArgString(parseToolArgs(item.args), "command").trim();
}

function shellTitle(command: string, running: boolean, t: ReturnType<typeof useT>): string {
  if (!command) return running ? t("shellCard.running") : t("shellCard.ran");
  const oneLine = command.replace(/\s+/g, " ").trim();
  if (oneLine.length <= 72) return oneLine;
  return `${oneLine.slice(0, 69)}…`;
}

function shellTags(command: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const match of command.matchAll(TAG_RE)) {
    const tag = match[0]!.toLowerCase();
    if (seen.has(tag)) continue;
    seen.add(tag);
    out.push(tag);
    if (out.length >= 4) break;
  }
  return out;
}

function shellBodyText(item: ToolItem): string {
  if (item.error) return item.error;
  return item.output ?? "";
}

export const ShellCommandCard = memo(function ShellCommandCard({ item }: { item: ToolItem }) {
  const t = useT();
  const shellExpand = useShellExpand();
  const running = item.status === "running";
  const command = shellCommand(item);
  const title = shellTitle(command, running, t);
  const tags = useMemo(() => shellTags(command), [command]);
  const body = shellBodyText(item);
  const hasBody = body.trim().length > 0;
  const [open, setOpen] = useState(running || hasBody);

  useEffect(() => {
    if (running) setOpen(true);
  }, [running]);

  useEffect(() => {
    if (!shellExpand) return;
    return shellExpand.register(item.id, () => setOpen((v) => !v));
  }, [item.id, shellExpand]);

  return (
    <div
      className={`shell-card shell-card--${item.status}${open ? " shell-card--open" : ""}${item.error ? " shell-card--error" : ""}`}
    >
      <button
        type="button"
        className="shell-card__head"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        <Terminal className="shell-card__icon" size={14} aria-hidden="true" />
        <span className="shell-card__title">{title}</span>
        {tags.length > 0 ? (
          <span className="shell-card__tags" aria-hidden="true">
            {tags.join(", ")}
          </span>
        ) : null}
        <span className="shell-card__meta">
          {running ? <Loader2 className="shell-card__spin" size={12} aria-hidden="true" /> : null}
          <ChevronRight className={`shell-card__chevron${open ? " shell-card__chevron--open" : ""}`} size={14} />
        </span>
      </button>
      <MotionUnfold open={open}>
        <div className="shell-card__body">
          {command ? <div className="shell-card__cmd">{command}</div> : null}
          {hasBody ? (
            <CodeViewer flat value={body} maxHeight={320} language={item.error ? undefined : "plaintext"} />
          ) : running ? (
            <div className="shell-card__waiting">{t("shellCard.waiting")}</div>
          ) : null}
          {item.truncated ? <div className="shell-card__note">{t("tool.truncated")}</div> : null}
        </div>
      </MotionUnfold>
    </div>
  );
});
