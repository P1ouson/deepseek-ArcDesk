import { useEffect, useRef } from "react";
import { Check, Circle, CircleDot, RefreshCw, X } from "lucide-react";
import { useT } from "../lib/i18n";
import type { Todo } from "../lib/tools";
import { Tooltip } from "./Tooltip";

export function TodoPanel({
  todos,
  stale,
  onDismiss,
  onStartPlan,
}: {
  todos: Todo[];
  stale?: boolean;
  onDismiss: () => void;
  onStartPlan?: () => void;
}) {
  const t = useT();
  const currentRef = useRef<HTMLDivElement | null>(null);

  const done = todos.filter((item) => item.status === "completed").length;
  const current = todos.find((item) => item.status === "in_progress");

  useEffect(() => {
    currentRef.current?.scrollIntoView({ block: "nearest" });
  }, [current?.content, current?.activeForm]);

  if (todos.length === 0) {
    return (
      <div className="dock-panel todo-panel">
        <header className="dock-panel__head">
          <div className="dock-panel__head-main">
            <h2 className="dock-panel__title">{t("rightDock.tab.todo")}</h2>
          </div>
        </header>
        <div className="dock-panel__empty dock-panel__empty--dock">
          <span>{t("rightDock.todoEmpty")}</span>
          <small>{t("rightDock.planHint")}</small>
          {onStartPlan && (
            <button type="button" className="todo-panel__start" onClick={onStartPlan}>
              {t("rightDock.planStart")}
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="dock-panel todo-panel">
      <header className="dock-panel__head">
        <div className="dock-panel__head-main">
          <h2 className="dock-panel__title">{t("todo.title")}</h2>
          <p className="dock-panel__meta">
            {done}/{todos.length}
            {stale && (
              <span className="todo-panel__stale">
                <RefreshCw size={11} />
                {t("todo.stale")}
              </span>
            )}
          </p>
        </div>
        <Tooltip label={t("todo.dismiss")}>
          <button type="button" className="dock-panel__ghost" onClick={onDismiss} aria-label={t("todo.dismiss")}>
            <X size={14} strokeWidth={1.75} />
          </button>
        </Tooltip>
      </header>

      {current && (
        <p className="todo-panel__current" role="status">
          {current.activeForm || current.content}
        </p>
      )}

      <ul className="dock-panel__list todo-panel__list">
        {todos.map((item, index) => (
          <li key={index}>
            <div
              ref={item.status === "in_progress" ? currentRef : undefined}
              className={`todo-panel__item todo-panel__item--${item.status}${item.level ? " todo-panel__item--sub" : ""}`}
            >
              {item.status === "completed" ? (
                <Check size={14} strokeWidth={1.75} className="todo-panel__ico todo-panel__ico--done" />
              ) : item.status === "in_progress" ? (
                <CircleDot size={14} strokeWidth={1.75} className="todo-panel__ico todo-panel__ico--active" />
              ) : (
                <Circle size={14} strokeWidth={1.75} className="todo-panel__ico" />
              )}
              <span className="todo-panel__text">
                {item.status === "in_progress" && item.activeForm ? item.activeForm : item.content}
              </span>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
