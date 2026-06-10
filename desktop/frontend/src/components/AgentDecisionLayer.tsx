import type { QuestionAnswer, WireApproval, WireAsk } from "../lib/types";
import { ApprovalModal } from "./ApprovalModal";
import { AskCard } from "./AskCard";

export function AgentDecisionLayer({
  approval,
  ask,
  surface,
  planToolCount,
  onApprove,
  onRevisePlan,
  onExitPlan,
  onAnswerAsk,
  onDismissAsk,
}: {
  approval: WireApproval | null | undefined;
  ask: WireAsk | null | undefined;
  surface: "code" | "write";
  planToolCount?: number;
  onApprove: (allow: boolean, session: boolean, persist: boolean) => void;
  onRevisePlan: (text: string) => void;
  onExitPlan: () => void;
  onAnswerAsk: (id: string, answers: QuestionAnswer[]) => void;
  onDismissAsk: () => void;
}) {
  if (!approval && !ask) return null;

  return (
    <div className="arc-decision-layer" data-surface={surface}>
      <div className="arc-decision-layer__inner">
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
    </div>
  );
}
