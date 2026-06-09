import { ArrowUp, MessageCircle, X } from "lucide-react";
import { useState } from "react";
import { useT } from "../lib/i18n";

export interface SideMessage {
  id: string;
  text: string;
  outgoing: boolean;
  createdAt: number;
  pending?: boolean;
}

export interface SideConversationProps {
  mainTitle: string;
  messages: SideMessage[];
  busy?: boolean;
  onSend: (text: string) => void;
  onClose: () => void;
}

export function SideConversation({ mainTitle, messages, busy = false, onSend, onClose }: SideConversationProps) {
  const t = useT();
  const [text, setText] = useState("");
  if (messages.length === 0) return null;

  const submit = () => {
    const trimmed = text.trim();
    if (!trimmed || busy) return;
    onSend(trimmed);
    setText("");
  };

  return (
    <aside className="side-conversation side-conversation--arc">
      <header className="side-conversation__head">
        <div>
          <div className="side-conversation__title">
            <MessageCircle size={15} /> {t("sideChat.title")}
          </div>
          <div className="side-conversation__context">{t("sideChat.context", { title: mainTitle })}</div>
          <div className="side-conversation__hint">{t("sideChat.hintLive")}</div>
        </div>
        <button type="button" onClick={onClose} aria-label={t("common.close")}>
          <X size={15} />
        </button>
      </header>
      <div className="side-conversation__list">
        {messages.map((message) => (
          <article
            key={message.id}
            className={`side-conversation__bubble${message.outgoing ? " side-conversation__bubble--user" : " side-conversation__bubble--agent"}${message.pending ? " side-conversation__bubble--pending" : ""}`}
          >
            {message.pending ? <span className="side-conversation__pending-dot" aria-hidden="true" /> : null}
            <span>{message.text}</span>
          </article>
        ))}
      </div>
      <div className="side-conversation__composer">
        <textarea
          value={text}
          disabled={busy}
          onChange={(e) => setText(e.target.value)}
          placeholder={t("sideChat.placeholder")}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              submit();
            }
          }}
        />
        <button
          type="button"
          className="side-conversation__send"
          disabled={!text.trim() || busy}
          onClick={submit}
          aria-label={t("sideChat.send")}
        >
          <ArrowUp size={16} />
        </button>
      </div>
    </aside>
  );
}
