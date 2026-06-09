import { createPortal } from "react-dom";
import type { QuestionAnswer, WireApproval, WireAsk } from "../lib/types";
import { ApprovalModal } from "./ApprovalModal";
import { AskCard } from "./AskCard";
import { DecisionCoach } from "./DecisionCoach";
import { shouldShowDecisionCoach } from "../lib/decisionCoach";

export function AgentDecisionLayer({
  approval,
  ask,
  mode,
  planToolCount,
  onApprove,
  onRevisePlan,
  onExitPlan,
  onAnswerAsk,
  onDismissAsk,
}: {
  approval: WireApproval | null | undefined;
  ask: WireAsk | null | undefined;
  mode: string;
  planToolCount?: number;
  onApprove: (allow: boolean, session: boolean, persist: boolean) => void;
  onRevisePlan: (text: string) => void;
  onExitPlan: () => void;
  onAnswerAsk: (id: string, answers: QuestionAnswer[]) => void;
  onDismissAsk: () => void;
}) {
  if (!approval && !ask) return null;

  const coachTopic = approval
    ? approval.tool === "exit_plan_mode"
      ? ("plan" as const)
      : ("approval" as const)
    : ask
      ? ("ask" as const)
      : null;

  const showCoach = coachTopic ? shouldShowDecisionCoach(coachTopic) : false;

  return createPortal(
    <div className="arc-decision-layer" data-mode={mode}>
      <div className="arc-decision-layer__inner">
        {showCoach && coachTopic ? <DecisionCoach topic={coachTopic} /> : null}
        {approval ? (
          <ApprovalModal
            approval={approval}
            planToolCount={planToolCount}
            onAnswer={onApprove}
            onRevisePlan={onRevisePlan}
            onExitPlan={onExitPlan}
          />
        ) : null}
        {ask ? <AskCard ask={ask} onAnswer={onAnswerAsk} onDismiss={onDismissAsk} /> : null}
      </div>
    </div>,
    document.body,
  );
}
