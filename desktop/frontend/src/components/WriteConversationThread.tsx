import { useEffect, useRef } from "react";
import { Loader2, User } from "lucide-react";
import { useT } from "../lib/i18n";
import type { WriteTurn } from "../lib/writeConversation";
import { Markdown } from "./Markdown";

export interface WriteConversationThreadProps {
  turns: WriteTurn[];
  running?: boolean;
  variant?: "panel" | "sidebar";
  emptyLabel?: string;
}

function previewText(text: string, max: number): string {
  const trimmed = text.trim();
  if (trimmed.length <= max) return trimmed;
  return `${trimmed.slice(0, max)}…`;
}

export function WriteConversationThread({
  turns,
  running = false,
  variant = "panel",
  emptyLabel,
}: WriteConversationThreadProps) {
  const t = useT();
  const endRef = useRef<HTMLDivElement>(null);
  const compact = variant === "sidebar";
  const streaming = turns.some((turn) => turn.streaming);
  const tailLen =
    turns.length > 0
      ? (turns[turns.length - 1]?.text.length ?? 0) + (turns[turns.length - 1]?.reasoning?.length ?? 0)
      : 0;

  useEffect(() => {
    if (variant !== "panel") return;
    endRef.current?.scrollIntoView({ block: "end", behavior: streaming ? "auto" : "smooth" });
  }, [turns, variant, streaming, tailLen]);

  if (!turns.length) {
    return (
      <div className={`write-conversation write-conversation--${variant} write-conversation--empty`}>
        <p>{emptyLabel ?? t("write.conversationEmpty")}</p>
      </div>
    );
  }

  return (
    <div className={`write-conversation write-conversation--${variant}`} role="log" aria-live="polite">
      {turns.map((turn) => (
        <article
          key={turn.id}
          className={`write-conversation__turn write-conversation__turn--${turn.role}${turn.streaming ? " write-conversation__turn--streaming" : ""}`}
        >
          <header className="write-conversation__head">
            <span className="write-conversation__role" aria-hidden="true">
              {turn.role === "user" ? <User size={13} /> : null}
              {turn.role === "user" ? t("write.conversationYou") : t("write.conversationAssistant")}
            </span>
            {turn.streaming ? (
              <span className="write-conversation__streaming">
                <Loader2 size={11} className="dock-panel__spin" />
              </span>
            ) : null}
          </header>
          {compact ? (
            <p className="write-conversation__preview">{previewText(turn.text, 72)}</p>
          ) : turn.role === "assistant" ? (
            <div className="write-conversation__body write-conversation__body--md">
              <Markdown text={turn.text || t("write.assistantStreaming")} streaming={turn.streaming} />
            </div>
          ) : (
            <p className="write-conversation__body">{turn.text}</p>
          )}
          {!compact && turn.reasoning?.trim() ? (
            <details className="write-conversation__reasoning" open={turn.streaming || undefined}>
              <summary>{t("write.reasoningTitle")}</summary>
              <div className="write-conversation__reasoning-body">{turn.reasoning}</div>
            </details>
          ) : null}
        </article>
      ))}
      {running && !turns.some((turn) => turn.streaming) ? (
        <div className="write-conversation__pending">
          <Loader2 size={12} className="dock-panel__spin" />
          <span>{t("write.assistantWorking")}</span>
        </div>
      ) : null}
      <div ref={endRef} />
    </div>
  );
}
