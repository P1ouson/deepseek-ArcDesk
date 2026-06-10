import { useEffect, useState } from "react";
import { app } from "../../lib/bridge";
import { normalizeLangPref, useI18n, useT, type LangPref } from "../../lib/i18n";
import type { AppMode } from "../../lib/appMode";
import {
  saveCodeReviewDefaultScope,
  saveCodeReviewSecurityByDefault,
  syncCodeReviewSettings,
  type CodeReviewDefaultScope,
} from "../../lib/codeReviewPrefs";
import type { DesktopCodeReviewView, NetworkView } from "../../lib/types";
import {
  SettingsActionButton,
  SettingsBlock,
  SettingsSaveChip,
  type SettingsSectionProps,
} from "../settingsPrimitives";
import { SettingsShortcutRow } from "./SettingsShortcutRow";
import { normalizeTerminalShell } from "./normalizeSettingsView";

const LANGUAGE_PREFS: LangPref[] = ["", "zh", "en"];
const TERMINAL_SHELL_PREFS = ["powershell", "cmd", "git-bash", "wsl"] as const;
const PROXY_MODES = ["auto", "env", "custom", "off"] as const;

function NetworkSettingsBlock({
  network,
  busy,
  apply,
}: {
  network: NetworkView;
  busy: boolean;
  apply: (fn: () => Promise<void>) => Promise<void>;
}) {
  const t = useT();
  const [draft, setDraft] = useState(network);

  useEffect(() => {
    setDraft(network);
  }, [network]);

  const patch = (next: Partial<NetworkView>) => setDraft((cur) => ({ ...cur, ...next }));

  return (
    <SettingsBlock title={t("settings.network")} hint={t("settings.proxyUrlHint")}>
      <div className="settings-block__form network-settings-form">
        <div className="set-seg set-seg--compact set-seg--wrap">
          {PROXY_MODES.map((mode) => (
            <button
              key={mode}
              type="button"
              className={`set-seg__btn${draft.proxyMode === mode ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => patch({ proxyMode: mode })}
            >
              {t(`settings.proxyMode.${mode}`)}
            </button>
          ))}
        </div>
        <div className="set-row set-row--stack">
          <label className="set-label">{t("settings.proxyUrl")}</label>
          <input
            className="mem-input"
            value={draft.proxyUrl}
            placeholder={t("settings.proxyUrlPlaceholder")}
            disabled={busy}
            onChange={(e) => patch({ proxyUrl: e.target.value })}
          />
        </div>
        <div className="set-row set-row--stack">
          <label className="set-label">{t("settings.noProxy")}</label>
          <input
            className="mem-input"
            value={draft.noProxy}
            placeholder={t("settings.noProxyPlaceholder")}
            disabled={busy}
            onChange={(e) => patch({ noProxy: e.target.value })}
          />
        </div>
        <div className="set-row">
          <label className="set-label">{t("settings.proxyServer")}</label>
          <input
            className="mem-input set-grow"
            value={draft.proxy.server}
            placeholder={t("settings.proxyServerPlaceholder")}
            disabled={busy}
            onChange={(e) => patch({ proxy: { ...draft.proxy, server: e.target.value } })}
          />
        </div>
        <div className="set-row">
          <label className="set-label">{t("settings.proxyPort")}</label>
          <input
            className="mem-input set-grow"
            type="number"
            value={draft.proxy.port || ""}
            placeholder={t("settings.proxyPortPlaceholder")}
            disabled={busy}
            onChange={(e) => patch({ proxy: { ...draft.proxy, port: Number(e.target.value) || 0 } })}
          />
        </div>
        <SettingsSaveChip disabled={busy} ready onClick={() => void apply(() => app.SetNetwork(draft))}>
          {t("settings.saveNetwork")}
        </SettingsSaveChip>
      </div>
    </SettingsBlock>
  );
}

function CodeReviewSettingsBlock({ s, busy, apply }: SettingsSectionProps) {
  const t = useT();
  const saved = s.desktopCodeReview ?? { defaultScope: "all", securityByDefault: false };
  const scope = (saved.defaultScope === "session" || saved.defaultScope === "git" ? saved.defaultScope : "all") as CodeReviewDefaultScope;
  const securityDefault = saved.securityByDefault === true;

  const persist = (next: DesktopCodeReviewView) => {
    syncCodeReviewSettings(next);
    void apply(() => app.SetDesktopCodeReviewSettings(next));
  };

  return (
    <SettingsBlock title={t("settings.general.codeReview")} hint={t("settings.general.codeReviewHint")}>
      <div className="settings-block__stack">
        <div>
          <div className="settings-field__label">{t("settings.general.codeReviewScope")}</div>
          <div className="set-seg set-seg--compact set-seg--wrap">
            {(["all", "session", "git"] as const).map((value) => (
              <button
                key={value}
                type="button"
                className={`set-seg__btn${scope === value ? " set-seg__btn--on" : ""}`}
                disabled={busy}
                onClick={() => {
                  saveCodeReviewDefaultScope(value);
                  persist({ defaultScope: value, securityByDefault: securityDefault });
                }}
              >
                {t(
                  value === "all"
                    ? "changes.filterAll"
                    : value === "session"
                      ? "changes.filterSession"
                      : "changes.filterGit",
                )}
              </button>
            ))}
          </div>
        </div>
        <label className="settings-block__check">
          <input
            type="checkbox"
            checked={securityDefault}
            disabled={busy}
            onChange={(event) => {
              saveCodeReviewSecurityByDefault(event.target.checked);
              persist({ defaultScope: scope, securityByDefault: event.target.checked });
            }}
          />
          <span>{t("settings.general.codeReviewSecurityDefault")}</span>
        </label>
      </div>
    </SettingsBlock>
  );
}

export function GeneralSection({
  s,
  busy,
  apply,
  onModeChange,
  onOpenCapabilities,
  onOpenOnboarding,
}: SettingsSectionProps & {
  onModeChange?: (mode: AppMode) => void;
  onOpenCapabilities?: () => void;
  onOpenOnboarding?: () => void;
}) {
  const { t, setPref } = useI18n();
  const languagePref = normalizeLangPref(s.desktopLanguage);
  const shellPref = normalizeTerminalShell(s.desktopTerminalShell) || "powershell";
  const setLanguage = (next: LangPref) => {
    setPref(next);
    void apply(() => app.SetDesktopLanguage(next));
  };

  return (
    <>
      <SettingsBlock title={t("settings.language")}>
        <div className="set-seg set-seg--compact">
          {LANGUAGE_PREFS.map((pref) => (
            <button
              key={pref || "auto"}
              type="button"
              className={`set-seg__btn${languagePref === pref ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => setLanguage(pref)}
            >
              {pref === "" ? t("settings.langAuto") : pref === "zh" ? "中文" : "English"}
            </button>
          ))}
        </div>
      </SettingsBlock>

      <NetworkSettingsBlock network={s.network} busy={busy} apply={apply} />

      <SettingsBlock title={t("settings.general.terminalShell")} hint={t("settings.general.terminalShellHint")}>
        <div className="set-seg set-seg--compact set-seg--wrap">
          {TERMINAL_SHELL_PREFS.map((shell) => (
            <button
              key={shell}
              type="button"
              className={`set-seg__btn${shellPref === shell ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => void apply(() => app.SetDesktopTerminalShell(shell))}
            >
              {t(
                shell === "powershell"
                  ? "settings.general.shell.powershell"
                  : shell === "cmd"
                    ? "settings.general.shell.cmd"
                    : shell === "git-bash"
                      ? "settings.general.shell.gitBash"
                      : "settings.general.shell.wsl",
              )}
            </button>
          ))}
        </div>
      </SettingsBlock>

      <CodeReviewSettingsBlock s={s} busy={busy} apply={apply} />

      <SettingsBlock title={t("settings.general.onboarding")} hint={t("settings.general.onboardingHint")}>
        <SettingsActionButton primary={false} disabled={busy || !onOpenOnboarding} onClick={() => onOpenOnboarding?.()}>
          {t("settings.general.onboarding")}
        </SettingsActionButton>
      </SettingsBlock>

      <SettingsBlock title={t("settings.integrations.lead")}>
        <div className="settings-block__stack settings-block__stack--shortcut-actions">
          <SettingsShortcutRow
            title={t("settings.integrations.openPlugins")}
            hint={t("settings.integrations.pluginsHint")}
            buttonLabel={t("settings.integrations.openPlugins")}
            onClick={onOpenCapabilities ?? (onModeChange ? () => onModeChange("plugins") : undefined)}
            disabled={busy}
          />
          <SettingsShortcutRow
            title={t("settings.integrations.openPhone")}
            hint={t("settings.integrations.phoneHint")}
            buttonLabel={t("settings.integrations.openPhone")}
            onClick={onModeChange ? () => onModeChange("phone") : undefined}
            disabled={busy}
          />
        </div>
      </SettingsBlock>
    </>
  );
}
