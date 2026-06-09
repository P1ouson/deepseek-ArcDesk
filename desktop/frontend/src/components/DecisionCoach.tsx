import { useState } from "react";
import { X } from "lucide-react";
import { markDecisionCoachSeen, type DecisionCoachTopic } from "../lib/decisionCoach";
import { useT } from "../lib/i18n";

export function DecisionCoach({ topic }: { topic: DecisionCoachTopic }) {
  const t = useT();
  const [visible, setVisible] = useState(true);

  if (!visible) return null;

  return (
    <div className="arc-coach" role="note">
      <div className="arc-coach__copy">
        <strong>{t(`decisionCoach.${topic}.title`)}</strong>
        <p>{t(`decisionCoach.${topic}.body`)}</p>
      </div>
      <button
        type="button"
        className="arc-coach__close"
        aria-label={t("decisionCoach.dismiss")}
        onClick={() => {
          markDecisionCoachSeen(topic);
          setVisible(false);
        }}
      >
        <X size={14} />
      </button>
    </div>
  );
}
