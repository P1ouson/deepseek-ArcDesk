import { useId } from "react";
import { useT } from "../lib/i18n";
import type { WireKnowledgeCapture } from "../lib/types";
import { PromptAction, PromptBadge, PromptShelf } from "./PromptShelf";

export function KnowledgeCaptureCard({
  capture,
  onRecord,
  onDismiss,
}: {
  capture: WireKnowledgeCapture;
  onRecord: () => void;
  onDismiss: () => void;
}) {
  const t = useT();
  const titleId = useId();
  const paths = capture.paths ?? [];

  return (
    <PromptShelf
      titleId={titleId}
      title={t("knowledge.capture.title")}
      badges={<PromptBadge>{t("knowledge.capture.badge")}</PromptBadge>}
      meta={capture.summary}
      actions={
        <>
          <PromptAction keyLabel="Y" label={t("knowledge.capture.record")} primary onClick={onRecord} />
          <PromptAction keyLabel="N" label={t("knowledge.capture.dismiss")} onClick={onDismiss} />
        </>
      }
    >
      {paths.length > 0 ? (
        <div className="knowledge-capture-card__paths">
          {paths.slice(0, 4).map((path) => (
            <code key={path}>{path}</code>
          ))}
        </div>
      ) : null}
    </PromptShelf>
  );
}
