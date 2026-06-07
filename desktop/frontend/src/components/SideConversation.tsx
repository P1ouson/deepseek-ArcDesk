import { X, MessageCircle } from "lucide-react";
import { useState } from "react";
import { useT } from "../lib/i18n";

export interface SideMessage {
  id: string;
  text: string;
  outgoing: boolean;
  createdAt: number;
}

export interface SideConversationProps {
  mainTitle: string;
  messages: SideMessage[];
  onSend: (text: string) => void;
  onClose: () => void;
}

export function SideConversation({ mainTitle, messages, onSend, onClose }: SideConversationProps) {
  const t = useT();
  const [text, setText] = useState("");
  if (messages.length === 0) return null;

  const submit = () => {
    const trimmed = text.trim();
    if (!trimmed) return;
    onSend(trimmed);
    setText("");
  };

  return (
    <aside className="side-conversation">
      <header className="side-conversation__head">
        <div>
          <div className="side-conversation__title">
            <MessageCircle size={15} /> {t("sideChat.title")}
          </div>
          <div className="side-conversation__context">{t("sideChat.context", { title: mainTitle })}</div>
          <div className="side-conversation__hint">{t("sideChat.hint")}</div>
        </div>
        <button type="button" onClick={onClose} aria-label={t("common.close")}>
          <X size={15} />
        </button>
      </header>
      <div className="side-conversation__list">
        {messages.map((message) => (
          <article
            key={message.id}
            className={`side-conversation__bubble${message.outgoing ? " side-conversation__bubble--user" : ""}`}
          >
            {message.text}
          </article>
        ))}
      </div>
      <div className="side-conversation__composer">
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder={t("sideChat.placeholder")}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              submit();
            }
          }}
        />
        <button type="button" disabled={!text.trim()} onClick={submit}>
          {t("sideChat.send")}
        </button>
      </div>
    </aside>
  );
}
