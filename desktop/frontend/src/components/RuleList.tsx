import { useState } from "react";
import { MotionUnfold } from "./MotionUnfold";
import { ChevronDown, Trash2 } from "lucide-react";
import { useT } from "../lib/i18n";
import { Tooltip } from "./Tooltip";

export function RuleList({
  list,
  rules,
  busy,
  onAdd,
  onRemove,
  title,
  tone,
  placeholder,
  collapsible = false,
  defaultOpen = false,
}: {
  list: string;
  rules: string[];
  busy: boolean;
  onAdd: (rule: string) => void | Promise<void>;
  onRemove: (rule: string) => void | Promise<void>;
  title?: string;
  tone?: string;
  placeholder?: string;
  collapsible?: boolean;
  defaultOpen?: boolean;
}) {
  const t = useT();
  const [draft, setDraft] = useState("");
  const [open, setOpen] = useState(defaultOpen);
  const add = () => {
    const r = draft.trim();
    if (!r || rules.includes(r)) return;
    void onAdd(r);
    setDraft("");
  };
  const label = title ?? list;
  const inputPlaceholder = placeholder ?? t("settings.addRule", { list });
  const toneClass = tone ? ` set-rules-fold__label--${tone}` : "";

  const body = (
    <>
      <div className="set-rules__list" role="list">
        {rules.length === 0 ? (
          <p className="set-rules__empty">{t("common.none")}</p>
        ) : (
          <ul className="set-rules__items">
            {rules.map((r) => (
              <li className="set-rules__item" key={r} role="listitem">
                <span className="set-rules__item-text">{r}</span>
                <Tooltip label={t("common.delete")}>
                  <button
                    type="button"
                    className="set-rules__item-remove"
                    disabled={busy}
                    aria-label={t("common.delete")}
                    onClick={() => void onRemove(r)}
                  >
                    <Trash2 size={13} aria-hidden="true" />
                  </button>
                </Tooltip>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="set-rules__composer">
        <input
          className="mem-input set-rules__input"
          placeholder={inputPlaceholder}
          value={draft}
          disabled={busy}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") add();
          }}
        />
        <button
          type="button"
          className="settings-action-btn settings-action-btn--compact set-rules__add-btn"
          disabled={busy || !draft.trim()}
          onClick={add}
        >
          {t("common.add")}
        </button>
      </div>
    </>
  );

  if (collapsible) {
    return (
      <div className="set-rules-fold">
        <button
          type="button"
          className={`settings-agent-advanced-toggle set-rules-fold__toggle${open ? " settings-agent-advanced-toggle--open" : ""}`}
          aria-expanded={open}
          onClick={() => setOpen((value) => !value)}
        >
          <ChevronDown size={13} aria-hidden="true" />
          <span className={`set-rules-fold__label${toneClass}`}>{label}</span>
        </button>
        <MotionUnfold open={open}>
          <div className="set-rules-fold__body">{body}</div>
        </MotionUnfold>
      </div>
    );
  }

  return (
    <div className={`set-rules-fold set-rules-fold--open${tone ? ` set-rules-fold--${tone}` : ""}`}>
      <div className="set-label set-rules-fold__label">{label}</div>
      <div className="set-rules-fold__body">{body}</div>
    </div>
  );
}
