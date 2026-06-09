import { CircleDot } from "lucide-react";
import { useT } from "../lib/i18n";
import type { QuestionAnswer, WireAsk } from "../lib/types";

export function AskTimelineBlock({
  ask,
  answers,
  dismissed,
  pending,
}: {
  ask: WireAsk;
  answers?: QuestionAnswer[];
  dismissed?: boolean;
  pending?: boolean;
}) {
  const t = useT();

  const answerFor = (questionId: string) => {
    const hit = answers?.find((a) => a.questionId === questionId);
    const labels = hit?.selected?.filter(Boolean) ?? [];
    return labels.length ? labels.join(", ") : null;
  };

  return (
    <div className={`msg-ask${pending ? " msg-ask--pending" : ""}`}>
      <div className="msg-ask__head">
        <CircleDot size={13} className="msg-ask__mark" aria-hidden />
        <span className="msg-ask__title">{t("ask.timelineTitle")}</span>
        {pending ? <span className="msg-ask__badge">{t("ask.timelinePending")}</span> : null}
        {dismissed ? <span className="msg-ask__badge msg-ask__badge--muted">{t("ask.timelineDismissed")}</span> : null}
      </div>
      <ul className="msg-ask__list">
        {ask.questions.map((question) => {
          const reply = answerFor(question.id);
          return (
            <li key={question.id} className="msg-ask__row">
              <span className="msg-ask__prompt">{question.header ? `${question.header} · ` : ""}{question.prompt}</span>
              {reply ? <span className="msg-ask__answer">{reply}</span> : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
