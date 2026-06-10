import { createPortal } from "react-dom";
import { useT } from "../lib/i18n";
import { hasGoBinding } from "../lib/runtime";

export function DevMockBanner() {
  const t = useT();
  if (hasGoBinding()) return null;
  return createPortal(
    <div className="dev-mock-banner" role="status">
      <strong>{t("devMock.title")}</strong>
      <span>{t("devMock.body")}</span>
    </div>,
    document.body,
  );
}
