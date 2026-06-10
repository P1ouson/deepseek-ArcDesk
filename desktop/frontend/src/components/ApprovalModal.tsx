import { useEffect, useRef, useState } from "react";
import { useT } from "../lib/i18n";
import type { WireApproval } from "../lib/types";
import { MotionUnfold } from "./MotionUnfold";
import { PromptAction, PromptDetailToggle, PromptShelf } from "./PromptShelf";

function truncateOneLine(text: string, max = 72): string {
  const line = text.trim().split("\n").find((entry) => entry.trim())?.trim() ?? "";
  if (line.length <= max) return line;
  return `${line.slice(0, max - 1)}…`;
}

export function ApprovalModal({
  approval,
  planToolCount,
  onAnswer,
  onRevisePlan,
  onExitPlan,
}: {
  approval: WireApproval;
  planToolCount?: number;
  onAnswer: (allow: boolean, session: boolean, persist: boolean) => void;
  onRevisePlan?: (text: string) => void;
  onExitPlan?: () => void;
}) {
  const t = useT();
  const [revisionOpen, setRevisionOpen] = useState(false);
  const [revisionText, setRevisionText] = useState("");
  const [detailsOpen, setDetailsOpen] = useState(false);
  const cardRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLTextAreaElement | null>(null);
  const isPlanApproval = approval.tool === "exit_plan_mode";
  const subject = approval.subject.trim();
  const subjectSummary = subject ? truncateOneLine(subject) : "";

  const choosePlanAction = (key: string) => {
    if (key === "1") setRevisionOpen((open) => !open);
    else if (key === "2") onAnswer(true, false, false);
    else if (key === "3" || key === "Escape") (onExitPlan ?? (() => onAnswer(false, false, false)))();
  };

  const chooseToolAction = (key: string) => {
    if (key === "1") onAnswer(true, false, false);
    else if (key === "2") onAnswer(true, true, false);
    else if (key === "3") onAnswer(true, true, true);
    else if (key === "4" || key === "Escape") onAnswer(false, false, false);
  };

  useEffect(() => {
    cardRef.current?.focus();
    setRevisionOpen(false);
    setRevisionText("");
    setDetailsOpen(false);
  }, [approval.id]);

  useEffect(() => {
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName.toLowerCase();
      if (tag === "input" || tag === "textarea" || target?.isContentEditable) return;
      if (event.key !== "1" && event.key !== "2" && event.key !== "3" && event.key !== "4" && event.key !== "Escape") return;
      event.preventDefault();
      if (isPlanApproval) choosePlanAction(event.key);
      else chooseToolAction(event.key);
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [isPlanApproval, onAnswer, onExitPlan]);

  useEffect(() => {
    if (revisionOpen) inputRef.current?.focus();
  }, [revisionOpen]);

  const submitRevision = () => {
    const text = revisionText.trim();
    if (!text) {
      inputRef.current?.focus();
      return;
    }
    onRevisePlan?.(text);
  };

  // The plan is already shown above as the assistant's reply; this is just the gate.
  if (isPlanApproval) {
    const planHint =
      typeof planToolCount === "number" && planToolCount > 0
        ? t("approval.planPreviewStats", { count: String(planToolCount) })
        : undefined;
    return (
      <PromptShelf
        barRef={cardRef}
        titleId="plan-approval-title"
        title={t("approval.planReady")}
        meta={t("approval.planReadyHint")}
        hint={planHint}
        actions={
          <>
            <PromptAction keyLabel="1" label={t("approval.revisePlan")} onClick={() => setRevisionOpen((open) => !open)} />
            <PromptAction keyLabel="2" label={t("approval.startExecution")} onClick={() => onAnswer(true, false, false)} selected />
            <PromptAction
              keyLabel="3"
              label={t("approval.exitPlan")}
              onClick={() => (onExitPlan ?? (() => onAnswer(false, false, false)))()}
            />
          </>
        }
      >
        <MotionUnfold open={revisionOpen}>
          <div className="plan-revision">
            <textarea
              ref={inputRef}
              className="plan-revision__input"
              value={revisionText}
              rows={3}
              placeholder={t("approval.revisePlanPlaceholder")}
              onChange={(event) => setRevisionText(event.target.value)}
              onKeyDown={(event) => {
                if ((event.metaKey || event.ctrlKey) && event.key === "Enter") submitRevision();
                event.stopPropagation();
              }}
            />
            <div className="plan-revision__actions">
              <button className="btn" onClick={() => setRevisionOpen(false)}>
                {t("common.cancel")}
              </button>
              <button className="btn btn--primary" onClick={submitRevision}>
                {t("approval.sendRevision")}
              </button>
            </div>
          </div>
        </MotionUnfold>
      </PromptShelf>
    );
  }

  return (
    <PromptShelf
      barRef={cardRef}
      titleId="tool-approval-title"
      title={t("approval.toolPending")}
      actionsWrap
      meta={
        <>
          <span className="tool__name">{approval.tool}</span>
          {subjectSummary && <span className="arc-decision__subject"> · {subjectSummary}</span>}
        </>
      }
      actions={
        <>
          {subject && (
            <PromptDetailToggle
              open={detailsOpen}
              label={t("approval.details")}
              openLabel={t("approval.hideDetails")}
              onClick={() => setDetailsOpen((open) => !open)}
            />
          )}
          <PromptAction keyLabel="1" label={t("approval.allowOnce")} onClick={() => onAnswer(true, false, false)} selected />
          <PromptAction keyLabel="2" label={t("approval.allowSession")} onClick={() => onAnswer(true, true, false)} />
          <PromptAction keyLabel="3" label={t("approval.allowPersistent")} onClick={() => onAnswer(true, true, true)} />
          <PromptAction keyLabel="4" label={t("approval.deny")} onClick={() => onAnswer(false, false, false)} />
        </>
      }
    >
      {subject ? (
        <MotionUnfold open={detailsOpen}>
          <pre className="approval-subject">{subject}</pre>
        </MotionUnfold>
      ) : null}
    </PromptShelf>
  );
}
