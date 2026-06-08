import { createPortal } from "react-dom";
import type { DictKey } from "../lib/i18n";
import { useT } from "../lib/i18n";
import type { GitHubCliProbeReason } from "../lib/gitHubCli";
import { probeReasonKey } from "../lib/gitHubCli";

export type GitHubCliSetupReason = GitHubCliProbeReason | "setup_required";

function setupMessageKey(reason: GitHubCliSetupReason, checkEnabled: boolean): DictKey {
  if (!checkEnabled) return "git.ghSetupNotConfigured";
  const key = probeReasonKey(reason === "setup_required" ? null : reason);
  return key ?? "git.ghSetupGeneric";
}

export function GitHubCliSetupModal({
  reason,
  checkEnabled,
  onClose,
  onOpenSettings,
}: {
  reason: GitHubCliSetupReason;
  checkEnabled: boolean;
  onClose: () => void;
  onOpenSettings: () => void;
}) {
  const t = useT();
  const messageKey = setupMessageKey(reason, checkEnabled);

  return createPortal(
    <div className="modal-backdrop" role="presentation" onClick={onClose}>
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="github-cli-setup-title"
        onClick={(event) => event.stopPropagation()}
      >
        <h3 className="modal__title" id="github-cli-setup-title">
          {t("git.ghSetupModalTitle")}
        </h3>
        <p className="modal__body">{t(messageKey)}</p>
        <div className="modal__actions">
          <button type="button" className="btn" onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button
            type="button"
            className="btn btn--primary"
            onClick={() => {
              onOpenSettings();
              onClose();
            }}
          >
            {t("git.ghSetupModalAction")}
          </button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
