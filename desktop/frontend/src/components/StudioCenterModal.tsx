import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { useT } from "../lib/i18n";

export function StudioCenterModal({
  title,
  titleId,
  children,
  onClose,
  wide = false,
  className,
}: {
  title: string;
  titleId?: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
  className?: string;
}) {
  const t = useT();
  const rootClass = [
    "modal",
    "studio-center-modal",
    wide ? "studio-center-modal--wide" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return createPortal(
    <div className="modal-backdrop modal-backdrop--static studio-center-modal-backdrop" role="presentation">
      <div
        className={rootClass}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId ?? "studio-center-modal-title"}
      >
        <button type="button" className="studio-center-modal__close" onClick={onClose} aria-label={t("common.close")}>
          <X size={16} strokeWidth={2} aria-hidden="true" />
        </button>
        <div className="studio-center-modal__head">
          <h3 className="studio-center-modal__title" id={titleId ?? "studio-center-modal-title"}>
            {title}
          </h3>
        </div>
        <div className="studio-center-modal__body">{children}</div>
      </div>
    </div>,
    document.body,
  );
}
