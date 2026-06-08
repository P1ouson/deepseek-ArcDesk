import { ArrowUp } from "lucide-react";
import { useT } from "../lib/i18n";
import { Tooltip } from "./Tooltip";

export function ComposerSendButton({
  disabled,
  onClick,
}: {
  disabled: boolean;
  onClick: () => void;
}) {
  const t = useT();
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
