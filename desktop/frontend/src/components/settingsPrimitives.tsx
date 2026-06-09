import { type ReactNode } from "react";
import { Check } from "lucide-react";

export type SettingsSectionProps = {
  s: import("../lib/types").SettingsView;
  busy: boolean;
  apply: (fn: () => Promise<void>) => Promise<void>;
};

export function SettingsPageShell({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="settings-page">
      <h2 className="settings-page__title">{title}</h2>
      <div className="settings-page__sections">{children}</div>
    </div>
  );
}

export function SettingsBlock({
  title,
  hint,
  children,
  compact,
}: {
  title: string;
  hint?: string;
  children: ReactNode;
  compact?: boolean;
}) {
  return (
    <section className={`settings-block${compact ? " settings-block--compact" : ""}`}>
      <h3 className="settings-block__title">{title}</h3>
      <div className="settings-block__card">
        {hint ? <p className="settings-block__card-lead">{hint}</p> : null}
        <div className="settings-block__card-content">{children}</div>
      </div>
    </section>
  );
}

export function SettingsSaveChip({
  children,
  onClick,
  disabled = false,
  ready = false,
}: {
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  ready?: boolean;
}) {
  return (
    <button
      type="button"
      className={`settings-save-chip${ready ? " settings-save-chip--ready" : ""}`}
      disabled={disabled}
      onClick={onClick}
    >
      <Check size={13} strokeWidth={2} aria-hidden="true" />
      {children}
    </button>
  );
}

export function SettingsActionButton({
  children,
  onClick,
  primary = true,
  disabled = false,
}: {
  children: ReactNode;
  onClick: () => void;
  primary?: boolean;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      className={`settings-action-btn${primary ? " settings-action-btn--primary" : ""}`}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </button>
  );
}
