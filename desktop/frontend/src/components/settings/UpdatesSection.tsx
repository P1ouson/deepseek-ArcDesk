import { useEffect, useState } from "react";
import { app } from "../../lib/bridge";
import { logBridgeError } from "../../lib/logBridgeError";
import { useT } from "../../lib/i18n";
import { useUpdater } from "../../lib/useUpdater";
import { Tooltip } from "../Tooltip";
import { SettingsBlock } from "../settingsPrimitives";

const MB = 1024 * 1024;
const mb = (n: number) => (n / MB).toFixed(1);

// UpdatesSection is the manual side of the auto-updater: it shows the running
// version and a Check button, then the same state machine the top banner uses
// (useUpdater) — available → install/download, with progress and errors inline.
export function UpdatesSection({ configPath }: { configPath: string }) {
  const t = useT();
  const { status, check, apply } = useUpdater();
  const [version, setVersion] = useState("");
  useEffect(() => {
    app.Version().then(setVersion).catch((err) => logBridgeError("Version", err));
  }, []);

  const busy =
    status.kind === "checking" || status.kind === "downloading" || status.kind === "verifying" || status.kind === "applying";

  return (
    <SettingsBlock title={t("updater.title")}>
      <div className="settings-block__form">
      <div className="set-row">
        <label className="set-label">{t("updater.currentVersion", { v: version || "…" })}</label>
        <span className="prov-card__spacer" />
        <button className="btn btn--small" disabled={busy} onClick={() => void check()}>
          {status.kind === "checking" ? t("updater.checking") : t("updater.checkButton")}
        </button>
      </div>
      {status.kind === "upToDate" && <div className="mem-hint">{t("updater.upToDate")}</div>}
      {status.kind === "available" && (
        <>
          <div className="set-row">
            <span className="set-label">{t("updater.available", { v: status.info.latest })}</span>
            <span className="prov-card__spacer" />
            <button className="btn btn--primary btn--small" onClick={() => apply(status.info)}>
              {status.info.canSelfUpdate ? t("updater.installNow") : t("updater.goToDownload")}
            </button>
          </div>
          {!status.info.canSelfUpdate && <div className="mem-hint">{t("updater.macHint")}</div>}
        </>
      )}
      {status.kind === "downloading" && (
        <div className="mem-hint">
          {t("updater.downloading", {
            done: mb(status.received),
            total: mb(status.total),
            pct: status.total > 0 ? Math.round((status.received / status.total) * 100) : 0,
          })}
        </div>
      )}
      {status.kind === "verifying" && <div className="mem-hint">{t("updater.verifying")}</div>}
      {status.kind === "applying" && <div className="mem-hint">{t("updater.applying")}</div>}
      {status.kind === "done" && <div className="mem-hint">{t("updater.done")}</div>}
      {status.kind === "error" && <div className="banner banner--error">{t("updater.failed", { msg: status.message })}</div>}
      {configPath && (
        <Tooltip label={configPath} fill block className="mem-hint settings-config-path">
          {t("settings.config", { path: configPath })}
        </Tooltip>
      )}
      </div>
    </SettingsBlock>
  );
}
