import { ArrowUp, Square } from "lucide-react";
import { useT } from "../lib/i18n";
import { Tooltip } from "./Tooltip";

export function ComposerSendButton({
  disabled,
  running = false,
  onClick,
  onCancel,
}: {
  disabled: boolean;
  running?: boolean;
  onClick: () => void;
  onCancel?: () => void;
}) {
  const t = useT();
  if (running) {
    return (
      <div className="composer-send-outer">
        <Tooltip label={t("composer.stop")}>
          <button
            className="composer__btn composer__btn--stop"
            type="button"
            onClick={onCancel}
            aria-label={t("composer.stopShort")}
          >
            <Square size={14} fill="currentColor" />
          </button>
        </Tooltip>
      </div>
    );
  }
  return (
    <div className="composer-send-outer">
      <Tooltip label={t("composer.send")}>
        <button className="composer__btn composer__btn--send" type="button" onClick={onClick} disabled={disabled}>
          <ArrowUp size={16} />
        </button>
      </Tooltip>
    </div>
  );
}
