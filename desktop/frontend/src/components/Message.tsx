import { memo, useState } from "react";
import { MotionUnfold } from "./MotionUnfold";
import { ChevronRight, MoreHorizontal } from "lucide-react";
import { Markdown } from "./Markdown";
import { CopyButton } from "./CopyButton";
import { Tooltip } from "./Tooltip";
import { useT } from "../lib/i18n";
import type { Item } from "../lib/useController";
import type { CheckpointMeta } from "../lib/types";

type AssistantItem = Extract<Item, { kind: "assistant" }>;

function MessageRewindMenu({
  turn,
  open,
  onToggle,
  onRewind,
  checkpoint,
  actionPending = false,
  rewindDisabled = false,
}: {
  turn: number;
  open?: boolean;
  onToggle?: () => void;
  onRewind?: (turn: number, scope: string) => void;
  checkpoint?: CheckpointMeta;
  actionPending?: boolean;
  rewindDisabled?: boolean;
}) {
  const t = useT();
  const [confirmScope, setConfirmScope] = useState<string | null>(null);

  const actionDisabledReason = (scope: string): string => {
    if (rewindDisabled || actionPending) return t("rewind.disabledRunning");
    if (!checkpoint) return t("rewind.disabledNoCheckpoint");
    if ((scope === "fork" || scope === "summ-from" || scope === "conversation") && !checkpoint.canConversation) {
      return t("rewind.disabledNoBoundary");
    }
    if (scope === "summ-upto") {
      if (!checkpoint.canConversation) return t("rewind.disabledNoBoundary");
      if (turn <= 0) return t("rewind.disabledNoEarlier");
    }
    if (scope === "code" && !checkpoint.canCode) return t("rewind.disabledNoCode");
    if (scope === "both") {
      if (!checkpoint.canConversation) return t("rewind.disabledNoBoundary");
      if (!checkpoint.canCode) return t("rewind.disabledNoCode");
    }
    return "";
  };

  const actionLabel = (scope: string): string => {
    if (confirmScope !== scope) {
      switch (scope) {
        case "fork":
          return t("rewind.fork");
        case "summ-from":
          return t("rewind.summFrom");
        case "summ-upto":
          return t("rewind.summUpto");
        case "conversation":
          return t("rewind.conversation");
        case "code":
          return t("rewind.code");
        default:
          return t("rewind.both");
      }
    }
    switch (scope) {
      case "fork":
        return t("rewind.confirmFork");
      case "summ-from":
        return t("rewind.confirmSummFrom");
      case "summ-upto":
        return t("rewind.confirmSummUpto");
      case "conversation":
        return t("rewind.confirmConversation");
      case "code":
        return t("rewind.confirmCode");
      default:
        return t("rewind.confirmBoth");
    }
  };

  const actionMeta = (scope: string): string => {
    if ((scope === "code" || scope === "both") && checkpoint?.files?.length) {
      return t("rewind.filesChanged", { count: checkpoint.files.length });
    }
    return "";
  };

  const runAction = (scope: string) => {
    setConfirmScope(null);
    onRewind?.(turn, scope);
  };

  const selectRewind = (scope: string) => {
    if (actionDisabledReason(scope)) return;
    if (confirmScope !== scope) {
      setConfirmScope(scope);
      return;
    }
    runAction(scope);
  };

  const renderAction = (scope: string, danger = false) => {
    const disabledReason = actionDisabledReason(scope);
    const meta = actionMeta(scope);
    return (
      <button
        className={[
          "rewind__menu-item",
          danger ? "rewind__menu-danger" : "",
          confirmScope === scope ? "rewind__menu-confirm" : "",
        ]
          .filter(Boolean)
          .join(" ")}
        type="button"
        disabled={Boolean(disabledReason)}
        title={disabledReason || undefined}
        onClick={() => selectRewind(scope)}
      >
        <span>{actionLabel(scope)}</span>
        {meta && <span className="rewind__menu-meta">{meta}</span>}
      </button>
    );
  };

  if (!onRewind) return null;

  return (
    <div className={`rewind rewind--footer${open ? " rewind--open" : ""}`}>
      <Tooltip label={t("rewind.label")}>
        <button
          className="msg-tool-btn motion-surface rewind__btn"
          type="button"
          aria-label={t("rewind.label")}
          aria-expanded={Boolean(open)}
          onClick={() => {
            setConfirmScope(null);
            onToggle?.();
          }}
        >
          <MoreHorizontal size={15} />
        </button>
      </Tooltip>
      {open && (
        <div className="rewind__menu rewind__menu--up">
          {rewindDisabled && <div className="rewind__menu-hint">{t("rewind.disabledRunning")}</div>}
          {!rewindDisabled && !checkpoint && <div className="rewind__menu-hint">{t("rewind.disabledNoCheckpoint")}</div>}
          {renderAction("conversation")}
          {renderAction("code")}
          {renderAction("both", true)}
        </div>
      )}
    </div>
  );
}

export function UserMessage({
  text,
  turn,
  anchorId,
}: {
  text: string;
  turn?: number;
  anchorId?: string;
}) {
  const displayText = text.replace(/@\.ARCDESK\/attachments\/[^\s]+/g, "[image]");
  return (
    <div className="msg msg--user" id={anchorId} data-question-anchor={anchorId} data-turn={turn}>
      <span className="msg__caret">›</span>
      <div className="msg__text">{displayText}</div>
    </div>
  );
}

export const AssistantMessage = memo(function AssistantMessage({
  item,
  turn,
  open,
  onToggle,
  onRewind,
  checkpoint,
  actionPending = false,
  rewindDisabled = false,
}: {
  item: AssistantItem;
  turn?: number;
  open?: boolean;
  onToggle?: () => void;
  onRewind?: (turn: number, scope: string) => void;
  checkpoint?: CheckpointMeta;
  actionPending?: boolean;
  rewindDisabled?: boolean;
}) {
  const t = useT();
  const [reasoningOpen, setReasoningOpen] = useState(false);
  const showActions = !item.streaming && Boolean(item.text);
  const canRewind = onRewind != null && turn != null;

  return (
    <div className="msg msg--assistant">
      {item.reasoning && (
        <div className="reasoning">
          <button className="reasoning__toggle" onClick={() => setReasoningOpen((v) => !v)}>
            <ChevronRight
              className={`reasoning__chevron ${reasoningOpen ? "reasoning__chevron--open" : ""}`}
              size={12}
            />
            {t("msg.thinking")}
          </button>
          <MotionUnfold open={reasoningOpen}>
            <div className="reasoning__body">{item.reasoning}</div>
          </MotionUnfold>
        </div>
      )}
      <div className="msg__body">
        {item.streaming ? (
          <div className="msg__stream">
            {item.text}
            <span className="cursor" />
          </div>
        ) : (
          <Markdown text={item.text} />
        )}
      </div>
      {showActions ? (
        <div className="msg__actions">
          <CopyButton text={item.text} variant="tool" />
          <span className="msg__actions-spacer" aria-hidden="true" />
          {canRewind ? (
            <MessageRewindMenu
              turn={turn}
              open={open}
              onToggle={onToggle}
              onRewind={onRewind}
              checkpoint={checkpoint}
              actionPending={actionPending}
              rewindDisabled={rewindDisabled}
            />
          ) : null}
        </div>
      ) : null}
    </div>
  );
});
