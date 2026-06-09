import { SettingsActionButton } from "../settingsPrimitives";

export function SettingsShortcutRow({
  title,
  hint,
  buttonLabel,
  onClick,
  disabled = false,
}: {
  title: string;
  hint: string;
  buttonLabel: string;
  onClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <div className="settings-shortcut-row">
      <div className="settings-shortcut-row__copy">
        <strong>{title}</strong>
        <p className="settings-block__note">{hint}</p>
      </div>
      <div className="settings-shortcut-row__action">
        <SettingsActionButton primary={false} disabled={disabled || !onClick} onClick={() => onClick?.()}>
          {buttonLabel}
        </SettingsActionButton>
      </div>
    </div>
  );
}
