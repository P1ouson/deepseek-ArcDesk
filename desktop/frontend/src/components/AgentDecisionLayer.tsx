import type { QuestionAnswer, WireApproval, WireAsk, WireKnowledgeCapture } from "../lib/types";
import { ApprovalModal } from "./ApprovalModal";
import { AskCard } from "./AskCard";
import { KnowledgeCaptureCard } from "./KnowledgeCaptureCard";

export function AgentDecisionLayer({
  approval,
  ask,
  knowledgeCapture,
  surface,
  planToolCount,
  onApprove,
  onRevisePlan,
  onExitPlan,
  onAnswerAsk,
  onDismissAsk,
  onRecordKnowledgeCapture,
  onDismissKnowledgeCapture,
}: {
  approval: WireApproval | null | undefined;
  ask: WireAsk | null | undefined;
  knowledgeCapture?: WireKnowledgeCapture | null;
  surface: "code" | "write";
  planToolCount?: number;
  onApprove: (allow: boolean, session: boolean, persist: boolean) => void;
  onRevisePlan: (text: string) => void;
  onExitPlan: () => void;
  onAnswerAsk: (id: string, answers: QuestionAnswer[]) => void;
  onDismissAsk: () => void;
  onRecordKnowledgeCapture?: () => void;
  onDismissKnowledgeCapture?: () => void;
}) {
  if (!approval && !ask && !knowledgeCapture) return null;

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
        {knowledgeCapture && onRecordKnowledgeCapture && onDismissKnowledgeCapture ? (
          <KnowledgeCaptureCard
            capture={knowledgeCapture}
            onRecord={onRecordKnowledgeCapture}
            onDismiss={onDismissKnowledgeCapture}
          />
        ) : null}
      </div>
    </div>
  );
}
