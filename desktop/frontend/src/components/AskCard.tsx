import { useEffect, useMemo, useRef, useState } from "react";
import { useT } from "../lib/i18n";
import type { QuestionAnswer, WireAsk, WireAskQuestion } from "../lib/types";
import { MotionUnfold } from "./MotionUnfold";
import { PromptBadge, PromptDetailToggle, PromptShelf } from "./PromptShelf";

// AskCard renders the `ask` tool as a compact prompt shelf near the composer.
export function AskCard({
  ask,
  onAnswer,
  onDismiss,
}: {
  ask: WireAsk;
  onAnswer: (id: string, answers: QuestionAnswer[]) => void;
  onDismiss: () => void;
}) {
  const t = useT();
  const [sel, setSel] = useState<Record<string, string[]>>({});
  const [custom, setCustom] = useState<Record<string, string>>({});
  const [active, setActive] = useState(0);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const shelfRef = useRef<HTMLDivElement | null>(null);
  const advanceTimer = useRef<number | null>(null);

  const questions = ask.questions;
  const q = questions[Math.min(active, questions.length - 1)];
  const isLast = active >= questions.length - 1;
  const progress = `${Math.min(active + 1, questions.length)}/${questions.length}`;
  const hasMultipleQuestions = questions.length > 1;
  const hasOptionDescriptions = q?.options.some((option) => option.description?.trim()) ?? false;

  useEffect(() => {
    shelfRef.current?.focus();
    setSel({});
    setCustom({});
    setActive(0);
    setDetailsOpen(false);
    if (advanceTimer.current != null) window.clearTimeout(advanceTimer.current);
  }, [ask.id]);

  useEffect(() => {
    return () => {
      if (advanceTimer.current != null) window.clearTimeout(advanceTimer.current);
    };
  }, []);

  const answersFrom = (
    nextSel: Record<string, string[]> = sel,
    nextCustom: Record<string, string> = custom,
  ): QuestionAnswer[] =>
    questions.map((question) => ({
      questionId: question.id,
      selected: nextCustom[question.id]?.trim() ? [nextCustom[question.id].trim()] : (nextSel[question.id] ?? []),
    }));

  const answerLabel = (question: WireAskQuestion) => {
    const typed = custom[question.id]?.trim();
    if (typed) return typed;
    return (sel[question.id] ?? []).join(", ");
  };

  const answered = (question: WireAskQuestion) =>
    (sel[question.id]?.length ?? 0) > 0 || (custom[question.id]?.trim() ?? "") !== "";

  const currentAnswered = q ? answered(q) : false;

  const finishOrAdvance = (nextSel = sel, nextCustom = custom) => {
    if (advanceTimer.current != null) {
      window.clearTimeout(advanceTimer.current);
      advanceTimer.current = null;
    }
    if (isLast) {
      onAnswer(ask.id, answersFrom(nextSel, nextCustom));
      return;
    }
    setDetailsOpen(false);
    setActive((i) => Math.min(i + 1, questions.length - 1));
  };

  const toggle = (question: WireAskQuestion, label: string) => {
    const nextCustom = { ...custom, [question.id]: "" };
    const cur = sel[question.id] ?? [];
    const nextSel = question.multi
      ? { ...sel, [question.id]: cur.includes(label) ? cur.filter((x) => x !== label) : [...cur, label] }
      : { ...sel, [question.id]: [label] };

    setCustom(nextCustom);
    setSel(nextSel);

    if (!question.multi) {
      if (advanceTimer.current != null) window.clearTimeout(advanceTimer.current);
      advanceTimer.current = window.setTimeout(() => finishOrAdvance(nextSel, nextCustom), 140);
    }
  };

  const setTyped = (question: WireAskQuestion, text: string) => {
    setCustom((c) => ({ ...c, [question.id]: text }));
    if (text.trim()) setSel((s) => ({ ...s, [question.id]: [] }));
  };

  const goBack = () => {
    if (advanceTimer.current != null) {
      window.clearTimeout(advanceTimer.current);
      advanceTimer.current = null;
    }
    setDetailsOpen(false);
    setActive((i) => Math.max(0, i - 1));
  };

  useEffect(() => {
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName.toLowerCase();
      if (tag === "input" || tag === "textarea" || target?.isContentEditable) return;

      if (event.key === "Escape") {
        event.preventDefault();
        onDismiss();
        return;
      }
      if ((event.key === "ArrowLeft" || event.key === "Backspace") && active > 0) {
        event.preventDefault();
        goBack();
        return;
      }

      const index = Number(event.key) - 1;
      if (!Number.isInteger(index) || index < 0 || index >= q.options.length) return;
      event.preventDefault();
      toggle(q, q.options[index].label);
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [active, custom, onDismiss, q, sel]);

  const answeredSummary = useMemo(
    () =>
      questions
        .slice(0, active)
        .map((question) => answerLabel(question))
        .filter(Boolean),
    [active, custom, questions, sel],
  );

  if (!q) return null;

  return (
    <PromptShelf
      barRef={shelfRef}
      titleId="ask-shelf-title"
      title={t("ask.title")}
      actionsWrap
      badges={
        <>
          {q.header ? <PromptBadge>{q.header}</PromptBadge> : null}
          {hasMultipleQuestions ? <PromptBadge>{t("ask.questionProgress", { progress })}</PromptBadge> : null}
        </>
      }
      meta={hasMultipleQuestions ? t("ask.questionProgress", { progress }) : undefined}
      actions={
        <>
          {active > 0 ? (
            <button type="button" className="arc-decision__btn arc-decision__btn--ghost" onClick={goBack}>
              {t("ask.back")}
            </button>
          ) : null}
          {q.multi ? (
            <button
              type="button"
              className="arc-decision__btn arc-decision__btn--primary"
              onClick={() => finishOrAdvance()}
              disabled={!currentAnswered}
            >
              {isLast ? t("common.submit") : t("ask.next")}
            </button>
          ) : null}
          {hasOptionDescriptions ? (
            <PromptDetailToggle
              open={detailsOpen}
              label={t("ask.details")}
              openLabel={t("ask.hideDetails")}
              onClick={() => setDetailsOpen((open) => !open)}
            />
          ) : null}
          <button type="button" className="arc-decision__btn arc-decision__btn--ghost" onClick={onDismiss}>
            {t("ask.justChat")}
          </button>
        </>
      }
      crumbs={
        answeredSummary.length > 0 ? (
          <div className="ask-card__crumbs">
            {answeredSummary.map((answer, index) => (
              <span className="ask-card__crumb" key={`${index}-${answer}`}>
                {index + 1}. {answer}
              </span>
            ))}
          </div>
        ) : null
      }
    >
      <div className="ask-card">
        <p className="ask-card__prompt">{q.prompt}</p>
        <div className="ask-card__options" role="listbox" aria-label={q.prompt}>
          {q.options.map((option, index) => {
            const selected = (sel[q.id] ?? []).includes(option.label);
            return (
              <button
                key={option.label}
                type="button"
                className={`ask-card__option${selected ? " ask-card__option--selected" : ""}`}
                aria-pressed={selected}
                onClick={() => toggle(q, option.label)}
              >
                <kbd className="ask-card__option-key">{index + 1}</kbd>
                <span className="ask-card__option-copy">
                  <span className="ask-card__option-label">{option.label}</span>
                  {option.description ? (
                    <span className="ask-card__option-desc">{option.description}</span>
                  ) : null}
                </span>
              </button>
            );
          })}
        </div>
        <div className="ask-card__custom-row">
          <input
            className="ask-card__custom"
            placeholder={t("ask.customPlaceholder")}
            value={custom[q.id] ?? ""}
            onChange={(e) => setTyped(q, e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && currentAnswered) finishOrAdvance();
              e.stopPropagation();
            }}
          />
          {(q.multi || custom[q.id]?.trim()) && (
            <button
              type="button"
              className="arc-decision__btn arc-decision__btn--primary"
              onClick={() => finishOrAdvance()}
              disabled={!currentAnswered}
            >
              {isLast ? t("common.submit") : t("ask.next")}
            </button>
          )}
        </div>
        {hasOptionDescriptions ? (
          <MotionUnfold open={detailsOpen}>
            <div className="ask-card__detail-list">
              {q.options.map((option) => (
                <div className="ask-card__detail" key={option.label}>
                  <span className="ask-card__detail-label">{option.label}</span>
                  {option.description ? (
                    <span className="ask-card__detail-desc">{option.description}</span>
                  ) : null}
                </div>
              ))}
            </div>
          </MotionUnfold>
        ) : null}
      </div>
    </PromptShelf>
  );
}
