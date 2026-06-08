import { Loader2, Shield, Sparkles } from "lucide-react";
import { useMemo } from "react";
import { useT } from "../lib/i18n";
import type { ParsedReview, ReviewMode, ReviewScope, ReviewVerdictTone } from "../lib/codeReview";
import { parseReviewMarkdown } from "../lib/codeReview";

export type CodeReviewStatus = "idle" | "running" | "done" | "error";

export interface CodeReviewState {
  status: CodeReviewStatus;
  mode: ReviewMode;
  scope: ReviewScope;
  text?: string;
  error?: string;
  finishedAt?: number;
}

interface CodeReviewSectionProps {
  scope: ReviewScope;
  fileCount: number;
  running: boolean;
  review: CodeReviewState;
  mode: ReviewMode;
  onScopeChange: (scope: ReviewScope) => void;
  onModeChange: (mode: ReviewMode) => void;
  onRun: () => void;
  onClear: () => void;
  onOpenFile?: (path: string) => void;
}

function verdictLabel(tone: ReviewVerdictTone, t: ReturnType<typeof useT>): string {
  switch (tone) {
    case "ok":
      return t("changes.reviewVerdictOk");
    case "warn":
      return t("changes.reviewVerdictWarn");
    case "block":
      return t("changes.reviewVerdictBlock");
    default:
      return t("changes.reviewVerdictUnknown");
  }
}

function severityLabel(severity: string, t: ReturnType<typeof useT>): string {
  switch (severity) {
    case "blocking":
    case "critical":
      return t("changes.reviewSeverityBlocking");
    case "high":
      return t("changes.reviewSeverityHigh");
    case "should-fix":
    case "medium":
      return t("changes.reviewSeverityShouldFix");
    case "nit":
      return t("changes.reviewSeverityNit");
    default:
      return t("changes.reviewSeverityInfo");
  }
}

function groupFindings(parsed: ParsedReview) {
  const groups = new Map<string, ParsedReview["findings"]>();
  for (const f of parsed.findings) {
    const key =
      f.severity === "critical" || f.severity === "blocking"
        ? "blocking"
        : f.severity === "high"
          ? "high"
          : f.severity === "should-fix" || f.severity === "medium"
            ? "should-fix"
            : f.severity === "nit"
              ? "nit"
              : "info";
    const list = groups.get(key) ?? [];
    list.push(f);
    groups.set(key, list);
  }
  return ["blocking", "high", "should-fix", "nit", "info"]
    .filter((k) => groups.has(k))
    .map((k) => ({ key: k, items: groups.get(k)! }));
}

export function CodeReviewSection({
  scope,
  fileCount,
  running,
  review,
  mode,
  onScopeChange,
  onModeChange,
  onRun,
  onClear,
  onOpenFile,
}: CodeReviewSectionProps) {
  const t = useT();
  const parsed = useMemo(
    () => (review.text ? parseReviewMarkdown(review.text) : null),
    [review.text],
  );
  const groups = parsed ? groupFindings(parsed) : [];

  const scopeOptions: { id: ReviewScope; labelKey: "changes.filterAll" | "changes.filterSession" | "changes.filterGit" | "changes.filterBoth" }[] = [
    { id: "all", labelKey: "changes.filterAll" },
    { id: "session", labelKey: "changes.filterSession" },
    { id: "git", labelKey: "changes.filterGit" },
    { id: "both", labelKey: "changes.filterBoth" },
  ];

  const busy = review.status === "running" || running;
  const canRun = fileCount > 0 && !busy;

  return (
    <section className="code-review" aria-label={t("changes.reviewTitle")}>
      <div className="dock-panel__section-head">
        <div className="dock-panel__section-title">
          <span>{t("changes.reviewTitle")}</span>
        </div>
        <button
          type="button"
          className="dock-panel__text-btn code-review__run"
          disabled={!canRun}
          onClick={onRun}
          title={fileCount === 0 ? t("changes.reviewNoFiles") : undefined}
        >
          {busy ? <Loader2 size={13} className="code-review__spin" /> : <Sparkles size={12} strokeWidth={1.75} />}
          {busy ? t("changes.reviewRunning") : t("changes.reviewRun")}
        </button>
      </div>

      <p className="code-review__hint">{t("changes.reviewHint")}</p>

      <div className="dock-panel__seg" role="tablist" aria-label={t("changes.reviewModeLabel")}>
        <button
          type="button"
          role="tab"
          aria-selected={mode === "standard"}
          className={`dock-panel__seg-btn${mode === "standard" ? " dock-panel__seg-btn--active" : ""}`}
          disabled={busy}
          onClick={() => onModeChange("standard")}
        >
          {t("changes.reviewModeStandard")}
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={mode === "security"}
          className={`dock-panel__seg-btn${mode === "security" ? " dock-panel__seg-btn--active" : ""}`}
          disabled={busy}
          onClick={() => onModeChange("security")}
        >
          <Shield size={11} strokeWidth={1.75} aria-hidden="true" />
          {t("changes.reviewModeSecurity")}
        </button>
      </div>

      <div className="dock-panel__seg code-review__scope" role="tablist" aria-label={t("changes.reviewScopeLabel")}>
        {scopeOptions.map(({ id, labelKey }) => (
          <button
            key={id}
            type="button"
            role="tab"
            aria-selected={scope === id}
            className={`dock-panel__seg-btn${scope === id ? " dock-panel__seg-btn--active" : ""}`}
            disabled={busy}
            onClick={() => onScopeChange(id)}
          >
            {t(labelKey)}
          </button>
        ))}
      </div>

      {review.status === "error" && review.error ? (
        <p className="code-review__banner code-review__banner--error">{review.error}</p>
      ) : null}

      {review.status === "idle" && !review.text ? (
        <p className="code-review__empty">{t("changes.reviewEmpty")}</p>
      ) : null}

      {parsed && review.status === "done" ? (
        <div className="code-review__result">
          <div className={`code-review__verdict code-review__verdict--${parsed.verdictTone}`}>
            <span className="code-review__verdict-label">{verdictLabel(parsed.verdictTone, t)}</span>
            {parsed.verdict ? <p className="code-review__verdict-text">{parsed.verdict}</p> : null}
          </div>

          {groups.length > 0 ? (
            <div className="code-review__findings">
              {groups.map(({ key, items }) => (
                <div key={key} className="code-review__group">
                  <div className="code-review__group-title">{severityLabel(key, t)}</div>
                  <ul className="code-review__group-list">
                    {items.map((item, idx) => (
                      <li key={`${key}-${idx}`} className="code-review__finding">
                        {item.file ? (
                          <button
                            type="button"
                            className="code-review__finding-file"
                            onClick={() => onOpenFile?.(item.file!)}
                          >
                            {item.file}
                            {item.line ? `:${item.line}` : ""}
                          </button>
                        ) : null}
                        <span className="code-review__finding-text">{item.text}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          ) : (
            <pre className="code-review__raw">{parsed.raw}</pre>
          )}

          <div className="code-review__foot">
            <button type="button" className="dock-panel__text-btn" onClick={onClear}>
              {t("changes.reviewClear")}
            </button>
          </div>
        </div>
      ) : null}

      {review.status === "running" ? (
        <p className="code-review__status">{t("changes.reviewRunningHint")}</p>
      ) : null}
    </section>
  );
}
