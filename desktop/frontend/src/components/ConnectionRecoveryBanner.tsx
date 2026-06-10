import { useT } from "../lib/i18n";

/** Inline CTA when the controller failed to connect (e.g. onboarding skipped). */
export function ConnectionRecoveryBanner({ onOpenSetup }: { onOpenSetup: () => void }) {
  const t = useT();
  return (
    <div className="connection-recovery" role="status">
      <span className="connection-recovery__text">{t("connectionRecovery.message")}</span>
      <button type="button" className="connection-recovery__action" onClick={onOpenSetup}>
        {t("connectionRecovery.action")}
      </button>
    </div>
  );
}
