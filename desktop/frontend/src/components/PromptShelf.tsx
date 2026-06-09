import type { ReactNode, RefObject } from "react";
import { CircleDot } from "lucide-react";

export function PromptShelf({
  titleId,
  title,
  badges,
  meta,
  actions,
  children,
  crumbs,
  quickActions,
  barRef,
  actionsWrap = false,
  hint,
}: {
  titleId: string;
  title: ReactNode;
  badges?: ReactNode;
  meta: ReactNode;
  actions: ReactNode;
  children?: ReactNode;
  crumbs?: ReactNode;
  quickActions?: ReactNode;
  barRef?: RefObject<HTMLDivElement>;
  actionsWrap?: boolean;
  hint?: ReactNode;
}) {
  return (
    <article className={`arc-decision${actionsWrap ? " arc-decision--wrap" : ""}`} aria-live="polite">
      <div
        ref={barRef}
        className="arc-decision__head"
        role="group"
        aria-labelledby={titleId}
        tabIndex={-1}
      >
        <div className="arc-decision__mark" aria-hidden="true">
          <CircleDot size={15} />
        </div>
        <div className="arc-decision__copy">
          <div id={titleId} className="arc-decision__title-row">
            <h2 className="arc-decision__title">{title}</h2>
            {badges ? <div className="arc-decision__badges">{badges}</div> : null}
          </div>
          <p className="arc-decision__meta">{meta}</p>
          {hint ? <p className="arc-decision__hint">{hint}</p> : null}
        </div>
        <div className="arc-decision__actions">{actions}</div>
      </div>
      {crumbs}
      {children ? <div className="arc-decision__body">{children}</div> : null}
      {quickActions}
    </article>
  );
}

export function PromptBadge({ children }: { children: ReactNode }) {
  return <span className="arc-decision__badge">{children}</span>;
}

export function PromptAction({
  keyLabel,
  label,
  onClick,
  primary = false,
  selected = false,
}: {
  keyLabel: string;
  label: ReactNode;
  onClick: () => void;
  primary?: boolean;
  selected?: boolean;
}) {
  return (
    <button
      type="button"
      className={`arc-decision__btn${primary || selected ? " arc-decision__btn--primary" : ""}`}
      onClick={onClick}
    >
      <kbd className="arc-decision__kbd">{keyLabel}</kbd>
      <span>{label}</span>
    </button>
  );
}

export function PromptDetailToggle({
  open,
  label,
  openLabel = label,
  onClick,
}: {
  open: boolean;
  label: ReactNode;
  openLabel?: ReactNode;
  onClick: () => void;
}) {
  return (
    <button type="button" className="arc-decision__detail-toggle" onClick={onClick}>
      <span>{open ? openLabel : label}</span>
    </button>
  );
}
