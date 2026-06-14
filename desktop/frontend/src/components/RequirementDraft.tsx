import { useState } from "react";
import { createPortal } from "react-dom";
import { X, Sparkles, ArrowRight } from "lucide-react";
import { useT } from "../lib/i18n";

const STEP_KEYS = ["sdd.background", "sdd.goal", "sdd.criteria"] as const;
const PLACEHOLDER_KEYS = ["sdd.backgroundPlaceholder", "sdd.goalPlaceholder", "sdd.criteriaPlaceholder"] as const;

export interface RequirementDraftProps {
  onClose: () => void;
  onGeneratePlan: (prompt: string) => void;
  onAiAssist: (stepText: string) => void;
}

export function RequirementDraft({ onClose, onGeneratePlan, onAiAssist }: RequirementDraftProps) {
  const t = useT();
  const [step, setStep] = useState(0);
  const [background, setBackground] = useState("");
  const [goal, setGoal] = useState("");
  const [criteria, setCriteria] = useState("");

  const values = [background, goal, criteria];
  const setters = [setBackground, setGoal, setCriteria];
  const canNext = values[step].trim().length > 0;

  const buildPrompt = () =>
    [
      t("sdd.generatedIntro"),
      `${t("sdd.background")}: ${background.trim()}`,
      `${t("sdd.goal")}: ${goal.trim()}`,
      `${t("sdd.criteria")}: ${criteria.trim()}`,
      t("sdd.generatedAsk"),
    ].join("\n\n");

  return createPortal(
    <div className="requirement-draft-overlay" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <div
        className="requirement-draft motion-fade-in"
        role="dialog"
        aria-modal="true"
        aria-labelledby="requirement-draft-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <header className="requirement-draft__head">
          <div>
            <div className="requirement-draft__title" id="requirement-draft-title">
              <Sparkles size={15} /> {t("sdd.title")}
            </div>
            <div className="requirement-draft__meta">
              {t("sdd.step", { current: step + 1, total: STEP_KEYS.length, label: t(STEP_KEYS[step]) })}
            </div>
          </div>
          <button type="button" className="requirement-draft__close btn btn--small" onClick={onClose} aria-label={t("common.close")}>
            <X size={15} />
          </button>
        </header>
        <div className="requirement-draft__body">
          <label>
            {t(STEP_KEYS[step])}
            <textarea
              value={values[step]}
              onChange={(e) => setters[step](e.target.value)}
              placeholder={t(PLACEHOLDER_KEYS[step])}
            />
          </label>
        </div>
        <footer className="requirement-draft__foot">
          <button type="button" className="btn btn--small" disabled={step === 0} onClick={() => setStep((v) => Math.max(0, v - 1))}>
            {t("sdd.back")}
          </button>
          <button type="button" className="btn btn--small" disabled={!canNext} onClick={() => onAiAssist(values[step].trim())}>
            {t("sdd.aiAssist")}
          </button>
          {step < STEP_KEYS.length - 1 ? (
            <button type="button" className="btn btn--small btn--primary" disabled={!canNext} onClick={() => setStep((v) => v + 1)}>
              {t("sdd.next")} <ArrowRight size={14} />
            </button>
          ) : (
            <button
              type="button"
              className="btn btn--small btn--primary"
              disabled={!background.trim() || !goal.trim() || !criteria.trim()}
              onClick={() => onGeneratePlan(buildPrompt())}
            >
              {t("sdd.generatePlan")} <ArrowRight size={14} />
            </button>
          )}
        </footer>
      </div>
    </div>,
    document.body,
  );
}
