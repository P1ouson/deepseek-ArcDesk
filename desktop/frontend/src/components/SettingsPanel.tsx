import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import {
  GITHUB_CLI_SETTINGS_EVENT,
  type GitHubCliSettingsNavDetail,
} from "../lib/gitHubCliSettingsNav";
import {
  Bot,
  Check,
  ChevronDown,
  Cpu,
  FolderGit2,
  Loader2,
  BarChart3,
  Palette,
  RefreshCw,
  Settings2,
  Shield,
} from "lucide-react";
import { asArray } from "../lib/array";
import { app, openExternal } from "../lib/bridge";
import { normalizeLangPref, useI18n, useT, type LangPref } from "../lib/i18n";
import { useUpdater } from "../lib/useUpdater";
import { useDismissOnOutsidePointerDown } from "../lib/useDismissOnOutsidePointerDown";
import {
  applyTheme,
  getResolvedTheme,
  getTheme,
  getThemeStyle,
  normalizeThemePreference,
  normalizeThemeStyleForTheme,
  stylesForTheme,
  type Theme,
  type ThemeStyle,
} from "../lib/theme";
import { TEXT_SIZES, getTextSize, isTextSize, type TextSize } from "../lib/textSize";
import {
  CODE_FONT_SIZES,
  DIFF_MARKER_STYLES,
  applyAppearancePrefs,
  appearanceViewFromCurrentState,
  backgroundColorFor,
  backgroundPresetsForTheme,
  foregroundColorFor,
  foregroundPresetsForBackground,
  isBackgroundPreset,
  isForegroundPreset,
  loadBackgroundPreset,
  loadCodeFontSize,
  loadDiffMarkerStyle,
  loadForegroundPreset,
  saveBackgroundPreset,
  saveCodeFontSize,
  saveDiffMarkerStyle,
  saveForegroundPreset,
  saveTextSize,
  type BackgroundPreset,
  type CodeFontSize,
  type DiffMarkerStyle,
  type ForegroundPreset,
} from "../lib/appearancePrefs";
import type { AppMode } from "../lib/appMode";
import type { RightDockTab } from "./Topbar";
import { normalizeDesktopGit, syncDesktopGitSettings } from "../lib/desktopGitPrefs";
import { probeGitHubCli, probeReasonKey, syncGitHubRepoMergeMethod, type GitHubCliProbe } from "../lib/gitHubCli";
import type { DesktopGitView, GitPRMergeMethod } from "../lib/types";
import {
  saveCodeReviewDefaultScope,
  saveCodeReviewSecurityByDefault,
  syncCodeReviewSettings,
  type CodeReviewDefaultScope,
} from "../lib/codeReviewPrefs";
import type { AgentSettingsInput, DesktopCodeReviewView, OutputStyleView, ProviderView, SettingsView } from "../lib/types";
import { ProjectPreviewSettings } from "./ProjectPreviewSettings";
import { RuleList } from "./RuleList";
import { StudioSelect } from "./StudioSelect";
import { Tooltip } from "./Tooltip";
import { UsageInsightsSettings } from "./UsageInsightsSettings";

type SettingsTab =
  | "general"
  | "appearance"
  | "workspace"
  | "models"
  | "usage"
  | "agent"
  | "permissions"
  | "updates";

const SETTINGS_TABS: SettingsTab[] = [
  "general",
  "appearance",
  "workspace",
  "models",
  "usage",
  "agent",
  "permissions",
  "updates",
];

export interface SettingsPageProps {
  onChanged: () => void;
  workspaceRoot?: string;
  onModeChange?: (mode: AppMode) => void;
  onOpenHistory?: () => void;
  onOpenMemory?: () => void;
  onOpenCapabilities?: () => void;
  onOpenTrash?: () => void;
  onConfigureProjectSandbox?: () => void;
  onOpenDockTab?: (tab: RightDockTab) => void;
  onOpenTerminal?: () => void;
}

// SettingsPage is the full-page settings surface (Codex / Claude Code style).
export function SettingsPage({
  onChanged,
}: SettingsPageProps) {
  const t = useT();
  const [s, setS] = useState<SettingsView | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [theme, setThemeState] = useState<Theme>(getTheme());
  const [themeStyle, setThemeStyleState] = useState<ThemeStyle>(() => getThemeStyle(getTheme()));
  const [textSize, setTextSizeState] = useState<TextSize>(getTextSize());
  const [backgroundPreset, setBackgroundPresetState] = useState<BackgroundPreset>(() => loadBackgroundPreset());
  const [foregroundPreset, setForegroundPresetState] = useState<ForegroundPreset>(() => loadForegroundPreset());
  const [codeFontSize, setCodeFontSizeState] = useState<CodeFontSize>(() => loadCodeFontSize());
  const [diffMarker, setDiffMarkerState] = useState<DiffMarkerStyle>(() => loadDiffMarkerStyle());
  const [tab, setTab] = useState<SettingsTab>("general");
  const [ghCliSetupRequest, setGhCliSetupRequest] = useState<(GitHubCliSettingsNavDetail & { id: number }) | null>(
    null,
  );
  const ghCliSetupSeq = useRef(0);

  const reload = async () => setS(normalizeSettingsView(await app.Settings().catch(() => null)));
  useEffect(() => {
    void reload();
  }, []);
  useEffect(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<GitHubCliSettingsNavDetail>).detail ?? {};
      ghCliSetupSeq.current += 1;
      setGhCliSetupRequest({
        runCheck: detail.runCheck !== false,
        enableCheck: detail.enableCheck !== false,
        id: ghCliSetupSeq.current,
      });
      setTab("workspace");
      requestAnimationFrame(() => {
        document.getElementById("settings-github-cli")?.scrollIntoView({ behavior: "smooth", block: "start" });
      });
    };
    window.addEventListener(GITHUB_CLI_SETTINGS_EVENT, handler);
    return () => window.removeEventListener(GITHUB_CLI_SETTINGS_EVENT, handler);
  }, []);
  useEffect(() => {
    if (!s) return;
    const nextTheme = normalizeThemePreference(s.desktopTheme);
    const nextStyle = normalizeThemeStyleForTheme(s.desktopThemeStyle, nextTheme);
    setThemeState(nextTheme);
    setThemeStyleState(nextStyle);
    applyTheme(nextTheme, nextStyle, { syncSurfaces: false });
  }, [s?.desktopTheme, s?.desktopThemeStyle]);

  useEffect(() => {
    if (!s?.desktopAppearance) return;
    const a = s.desktopAppearance;
    if (isBackgroundPreset(a.backgroundPreset)) setBackgroundPresetState(a.backgroundPreset);
    if (isForegroundPreset(a.foregroundPreset)) setForegroundPresetState(a.foregroundPreset);
    if (isTextSize(a.textSize)) setTextSizeState(a.textSize);
    if (CODE_FONT_SIZES.includes(a.codeFontSize as CodeFontSize)) setCodeFontSizeState(a.codeFontSize as CodeFontSize);
    if (DIFF_MARKER_STYLES.includes(a.diffMarker as DiffMarkerStyle)) setDiffMarkerState(a.diffMarker as DiffMarkerStyle);
  }, [s?.desktopAppearance]);

  const persistAppearanceConfig = () => app.SetDesktopAppearancePrefs(appearanceViewFromCurrentState());

  // apply runs a mutation, re-reads settings, and refreshes the topbar/model. A
  // rejected binding (validation / rebuild failure) surfaces as an inline banner.
  const apply = async (fn: () => Promise<void>) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
      await reload();
      onChanged();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="settings-studio-shell">
      <aside className="settings-studio__sidebar">
        <div className="settings-studio__sidebar-head">
          <h2>{t("settings.title")}</h2>
          <p>{s?.configPath ? t("settings.configPath", { path: s.configPath }) : t("settings.subtitle")}</p>
        </div>
        <nav className="settings-studio__nav" aria-label={t("settings.title")}>
          {SETTINGS_TABS.map((id) => {
            const Icon = settingsTabIcon(id);
            return (
              <button
                key={id}
                type="button"
                className={`settings-studio__nav-item${tab === id ? " settings-studio__nav-item--active" : ""}`}
                onClick={() => setTab(id)}
              >
                <Icon size={15} />
                <span className="settings-studio__nav-copy">
                  <strong>{settingsTabLabel(id, t)}</strong>
                </span>
              </button>
            );
          })}
        </nav>
      </aside>

      <main className="settings-studio__main">
        {!s ? (
          <div className="empty settings-studio__loading">{t("settings.loading")}</div>
        ) : (
          <div className="settings-studio__scroll">
            <div className="settings-studio__inner">
              {err ? <div className="banner banner--error">{err}</div> : null}
              <SettingsPageShell title={settingsTabLabel(tab, t)}>
              {tab === "general" && <GeneralSection s={s} busy={busy} apply={apply} />}
            {tab === "appearance" && (
              <AppearanceSection
                theme={theme}
                themeStyle={themeStyle}
                backgroundPreset={backgroundPreset}
                foregroundPreset={foregroundPreset}
                textSize={textSize}
                codeFontSize={codeFontSize}
                diffMarker={diffMarker}
                onTheme={(nextTheme) => {
                  const nextStyle = normalizeThemeStyleForTheme(themeStyle, nextTheme);
                  applyTheme(nextTheme, nextStyle, { syncSurfaces: true });
                  setBackgroundPresetState(loadBackgroundPreset());
                  setForegroundPresetState(loadForegroundPreset());
                  setThemeState(nextTheme);
                  setThemeStyleState(nextStyle);
                  void apply(async () => {
                    await app.SetDesktopAppearance(nextTheme, nextStyle);
                    await persistAppearanceConfig();
                  });
                }}
                onThemeStyle={(nextStyle) => {
                  applyTheme(theme, nextStyle, { syncSurfaces: false });
                  setThemeStyleState(nextStyle);
                  void apply(() => app.SetDesktopAppearance(theme, nextStyle));
                }}
                onBackgroundPreset={(preset) => {
                  const nextForeground = saveBackgroundPreset(preset);
                  setBackgroundPresetState(preset);
                  setForegroundPresetState(nextForeground);
                  applyAppearancePrefs();
                  void apply(persistAppearanceConfig);
                }}
                onForegroundPreset={(preset) => {
                  const nextForeground = saveForegroundPreset(preset);
                  setForegroundPresetState(nextForeground);
                  applyAppearancePrefs();
                  void apply(persistAppearanceConfig);
                }}
                onTextSize={(size) => {
                  saveTextSize(size);
                  applyAppearancePrefs();
                  setTextSizeState(size);
                  void apply(persistAppearanceConfig);
                }}
                onCodeFontSize={(size) => {
                  saveCodeFontSize(size);
                  setCodeFontSizeState(size);
                  applyAppearancePrefs();
                  void apply(persistAppearanceConfig);
                }}
                onDiffMarker={(style) => {
                  saveDiffMarkerStyle(style);
                  setDiffMarkerState(style);
                  applyAppearancePrefs();
                  void apply(persistAppearanceConfig);
                }}
              />
            )}
            {tab === "workspace" && (
              <WorkspaceSection
                s={s}
                busy={busy}
                apply={apply}
                ghCliSetupRequest={ghCliSetupRequest}
                onGhCliSetupHandled={() => setGhCliSetupRequest(null)}
              />
            )}
            {tab === "models" && <ModelsServicesSection s={s} busy={busy} apply={apply} />}
            {tab === "usage" && <UsageInsightsSettings />}
            {tab === "agent" && <AgentSection s={s} busy={busy} apply={apply} onNavigateTab={setTab} />}
            {tab === "permissions" && <PermissionsSection s={s} busy={busy} apply={apply} />}
            {tab === "updates" && <UpdatesSection configPath={s.configPath} />}
              </SettingsPageShell>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}

type SectionProps = {
  s: SettingsView;
  busy: boolean;
  apply: (fn: () => Promise<void>) => Promise<void>;
};

function SettingsPageShell({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="settings-page">
      <h2 className="settings-page__title">{title}</h2>
      <div className="settings-page__sections">{children}</div>
    </div>
  );
}

function SettingsBlock({
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

function SettingsSaveChip({
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

function SettingsActionButton({
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

function settingsTabLabel(id: SettingsTab, t: ReturnType<typeof useT>): string {
  switch (id) {
    case "general":
      return t("settings.tab.general");
    case "appearance":
      return t("settings.tab.appearance");
    case "workspace":
      return t("settings.tab.workspace");
    case "models":
      return t("settings.tab.models");
    case "usage":
      return t("settings.tab.usage");
    case "agent":
      return t("settings.tab.agent");
    case "permissions":
      return t("settings.tab.permissions");
    case "updates":
      return t("settings.tab.updates");
  }
}

function settingsTabIcon(id: SettingsTab) {
  switch (id) {
    case "general":
      return Settings2;
    case "appearance":
      return Palette;
    case "workspace":
      return FolderGit2;
    case "models":
      return Cpu;
    case "usage":
      return BarChart3;
    case "agent":
      return Bot;
    case "permissions":
      return Shield;
    case "updates":
      return RefreshCw;
  }
}

// toRef normalises a stored model id (a provider name, a bare model, or a ref) to
// a "provider/model" ref so a <select> of refs can show it selected.
function toRef(model: string, s: SettingsView): string {
  if (!model) return "";
  if (model.includes("/")) return model;
  const byName = s.providers.find((p) => p.name === model);
  if (byName) return `${byName.name}/${byName.default || byName.models[0] || ""}`;
  const byModel = s.providers.find((p) => p.models.includes(model));
  if (byModel) return `${byModel.name}/${model}`;
  return model;
}

const LANGUAGE_PREFS: LangPref[] = ["", "zh", "en"];
const AUTO_PLAN_MODES = ["off", "on"] as const;

type AutoPlanMode = (typeof AUTO_PLAN_MODES)[number];

function normalizeAutoPlan(mode: string | undefined): AutoPlanMode {
  return mode === "ask" || mode === "on" ? "on" : "off";
}

function normalizeSettingsView(view: SettingsView | null | undefined): SettingsView | null {
  if (!view) return null;
  const permissions = view.permissions ?? { mode: "ask", allow: [], ask: [], deny: [] };
  const sandbox = view.sandbox ?? { bash: "enforce", network: false, workspaceRoot: "", allowWrite: [] };
  const network = view.network ?? {
    proxyMode: "auto",
    proxyUrl: "",
    noProxy: "",
    proxy: { type: "socks5", server: "", port: 0, username: "", password: "" },
  };
  const rawAgent = view.agent;
  const agent = {
    temperature: rawAgent?.temperature ?? 0,
    maxSteps: rawAgent?.maxSteps ?? 0,
    systemPrompt: rawAgent?.systemPrompt ?? "",
    systemPromptFile: rawAgent?.systemPromptFile ?? "",
    outputStyle: rawAgent?.outputStyle ?? "",
    autoPlan: normalizeAutoPlan(rawAgent?.autoPlan ?? view.autoPlan),
    autoPlanClassifier: rawAgent?.autoPlanClassifier ?? "",
    softCompactRatio: rawAgent?.softCompactRatio ?? 0.5,
    compactRatio: rawAgent?.compactRatio ?? 0.8,
    compactForceRatio: rawAgent?.compactForceRatio ?? 0.9,
    subagentModel: rawAgent?.subagentModel ?? "",
    subagentModels: { ...(rawAgent?.subagentModels ?? {}) },
    usesDefaultPrompt: rawAgent?.usesDefaultPrompt ?? false,
    defaultSystemPrompt: rawAgent?.defaultSystemPrompt ?? "",
  };
  return {
    ...view,
    providers: asArray(view.providers).map((p) => ({ ...p, models: asArray(p.models) })),
    providerKinds: asArray(view.providerKinds),
    permissions: {
      ...permissions,
      allow: asArray(permissions.allow),
      ask: asArray(permissions.ask),
      deny: asArray(permissions.deny),
    },
    sandbox: {
      ...sandbox,
      allowWrite: asArray(sandbox.allowWrite),
    },
    network: {
      ...network,
      proxy: network.proxy ?? { type: "socks5", server: "", port: 0, username: "", password: "" },
    },
    agent,
    autoPlan: agent.autoPlan,
    desktopLanguage: normalizeLangPref(view.desktopLanguage),
    desktopTheme: normalizeThemePreference(view.desktopTheme),
    desktopThemeStyle: normalizeThemeStyleForTheme(view.desktopThemeStyle, normalizeThemePreference(view.desktopTheme)),
    desktopTerminalShell: normalizeTerminalShell(view.desktopTerminalShell),
    desktopGit: normalizeDesktopGit(view.desktopGit),
    desktopAppearance: view.desktopAppearance ?? {
      backgroundPreset: "",
      foregroundPreset: "",
      textSize: "default",
      codeFontSize: "default",
      diffMarker: "background",
    },
    desktopCodeReview: view.desktopCodeReview ?? { defaultScope: "all", securityByDefault: false },
    closeBehavior: normalizeCloseBehavior(view.closeBehavior),
  };
}

const GIT_PR_MERGE_METHODS: GitPRMergeMethod[] = ["merge", "squash", "rebase"];

const DEEPSEEK_OFFICIAL_BASE = "https://api.deepseek.com";
const TERMINAL_SHELL_PREFS = ["powershell", "cmd", "git-bash", "wsl"] as const;
type TerminalShellPref = (typeof TERMINAL_SHELL_PREFS)[number];

function normalizeTerminalShell(value: string | undefined): TerminalShellPref | "" {
  switch (value) {
    case "powershell":
    case "cmd":
    case "git-bash":
    case "wsl":
      return value;
    default:
      return "";
  }
}

function deepseekProviders(providers: ProviderView[]): ProviderView[] {
  return asArray(providers).filter((p) => p.apiKeyEnv === "DEEPSEEK_API_KEY");
}

type CloseBehavior = "background" | "quit";

function normalizeCloseBehavior(mode: string | undefined): CloseBehavior {
  return mode === "quit" ? "quit" : "background";
}

function GeneralSection({
  s,
  busy,
  apply,
}: SectionProps) {
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
        <button type="button" className="settings-action-btn settings-action-btn--compact" disabled title={t("settings.general.onboardingSoon")}>
          {t("settings.general.onboarding")}
        </button>
      </SettingsBlock>
    </>
  );
}

function CodeReviewSettingsBlock({ s, busy, apply }: SectionProps) {
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

function DeepSeekServiceUrlField({
  providers,
  busy,
  apply,
}: {
  providers: ProviderView[];
  busy: boolean;
  apply: SectionProps["apply"];
}) {
  const t = useT();
  const targets = deepseekProviders(providers);
  const currentBase = targets[0]?.baseUrl ?? DEEPSEEK_OFFICIAL_BASE;
  const [value, setValue] = useState(currentBase === DEEPSEEK_OFFICIAL_BASE ? "" : currentBase);

  useEffect(() => {
    const next = targets[0]?.baseUrl ?? DEEPSEEK_OFFICIAL_BASE;
    setValue(next === DEEPSEEK_OFFICIAL_BASE ? "" : next);
  }, [targets[0]?.baseUrl]);

  const savedDisplay =
    (targets[0]?.baseUrl ?? DEEPSEEK_OFFICIAL_BASE) === DEEPSEEK_OFFICIAL_BASE
      ? ""
      : (targets[0]?.baseUrl ?? "");
  const dirty = value !== savedDisplay;

  const save = () => {
    const next = value.trim() || DEEPSEEK_OFFICIAL_BASE;
    void apply(async () => {
      for (const provider of targets) {
        await app.SaveProvider({ ...provider, baseUrl: next });
      }
    });
  };

  return (
    <div className="settings-block__inline-save">
      <input
        className="mem-input settings-block__input"
        value={value}
        disabled={busy || !targets.length}
        placeholder={t("settings.models.baseUrlPlaceholder")}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && dirty) save();
        }}
      />
      <SettingsSaveChip disabled={busy || !targets.length || !dirty} ready={dirty} onClick={save}>
        {t("common.save")}
      </SettingsSaveChip>
    </div>
  );
}

const GITHUB_CLI_INSTALL_URL = "https://cli.github.com";

function GitHubCliStatusRow({
  label,
  status,
  detail,
  action,
}: {
  label: string;
  status: "pending" | "ok" | "warn" | "idle";
  detail: string;
  action?: ReactNode;
}) {
  const t = useT();
  return (
    <div className="settings-gh-status__row">
      <span className="settings-gh-status__label">{label}</span>
      <span className="settings-gh-status__detail">{detail}</span>
      <span
        className={`settings-block__status${status === "warn" ? " settings-block__status--warn" : ""}${status === "pending" || status === "idle" ? " settings-block__status--muted" : ""}`}
      >
        {status === "pending"
          ? "…"
          : status === "ok"
            ? t("settings.git.ghStatusOk")
            : status === "warn"
              ? t("settings.git.ghStatusFail")
              : "—"}
      </span>
      {action ? <span className="settings-gh-status__action">{action}</span> : null}
    </div>
  );
}

function GitHubCliSettingsBlock({
  gitDraft,
  busy,
  onChange,
  setupRequest,
  onSetupHandled,
}: {
  gitDraft: DesktopGitView;
  busy: boolean;
  onChange: (next: DesktopGitView) => void;
  setupRequest: (GitHubCliSettingsNavDetail & { id: number }) | null;
  onSetupHandled: () => void;
}) {
  const t = useT();
  const [probe, setProbe] = useState<GitHubCliProbe | null>(null);
  const [probing, setProbing] = useState(false);

  const runProbe = useCallback(async () => {
    setProbing(true);
    try {
      const result = await probeGitHubCli((command) => app.RunShellQuiet(command));
      setProbe(result);
    } finally {
      setProbing(false);
    }
  }, []);

  useEffect(() => {
    if (!gitDraft.checkGitHubCli) {
      setProbe(null);
      return;
    }
    void runProbe();
  }, [gitDraft.checkGitHubCli, runProbe]);

  useEffect(() => {
    if (!setupRequest) return;
    let cancelled = false;
    void (async () => {
      if (setupRequest.enableCheck && !gitDraft.checkGitHubCli) {
        onChange({ ...gitDraft, checkGitHubCli: true });
      }
      if (setupRequest.runCheck !== false) {
        await runProbe();
      }
      if (!cancelled) onSetupHandled();
    })();
    return () => {
      cancelled = true;
    };
  }, [setupRequest?.id]);

  const copyAuthCommand = () => {
    void navigator.clipboard?.writeText("gh auth login");
  };

  return (
    <SettingsBlock title={t("settings.git.githubCliTitle")} compact>
      <div className="settings-block__form settings-gh-block" id="settings-github-cli">
        <label className="set-check">
          <input
            type="checkbox"
            checked={gitDraft.checkGitHubCli}
            disabled={busy}
            onChange={(event) => onChange({ ...gitDraft, checkGitHubCli: event.target.checked })}
          />
          {t("settings.git.checkGitHubCli")}
        </label>
        {gitDraft.checkGitHubCli ? (
          <>
            <label className="set-check">
              <input
                type="checkbox"
                checked={gitDraft.syncRepoMergeToGitHub}
                disabled={busy}
                onChange={(event) => onChange({ ...gitDraft, syncRepoMergeToGitHub: event.target.checked })}
              />
              {t("settings.git.syncRepoMergeToGitHub")}
            </label>
            <div className="settings-gh-status" aria-live="polite">
              <GitHubCliStatusRow
                label={t("settings.git.ghStatusInstall")}
                status={probing ? "pending" : !probe ? "idle" : probe.ghInstalled ? "ok" : "warn"}
                detail={
                  probe?.ghVersion ??
                  (probe && !probe.ghInstalled ? t("settings.git.ghNotInstalled") : t("settings.git.ghStatusUnknown"))
                }
                action={
                  probe && !probe.ghInstalled ? (
                    <button type="button" className="settings-gh-status__link" onClick={() => openExternal(GITHUB_CLI_INSTALL_URL)}>
                      {t("settings.git.ghInstallLink")}
                    </button>
                  ) : null
                }
              />
              <GitHubCliStatusRow
                label={t("settings.git.ghStatusAuth")}
                status={
                  probing ? "pending" : !probe || !probe.ghInstalled ? "idle" : probe.ghAuthenticated ? "ok" : "warn"
                }
                detail={
                  probe?.ghAuthenticated
                    ? t("settings.git.ghLoggedIn")
                    : probe && probe.ghInstalled
                      ? t("settings.git.ghNotLoggedIn")
                      : t("settings.git.ghStatusUnknown")
                }
                action={
                  probe && probe.ghInstalled && !probe.ghAuthenticated ? (
                    <button type="button" className="settings-gh-status__link" onClick={copyAuthCommand}>
                      {t("settings.git.ghCopyAuthCommand")}
                    </button>
                  ) : null
                }
              />
              <GitHubCliStatusRow
                label={t("settings.git.ghStatusPR")}
                status={
                  probing
                    ? "pending"
                    : !probe || !probe.ghAuthenticated
                      ? "idle"
                      : probe.canMerge
                        ? "ok"
                        : "warn"
                }
                detail={
                  probe?.canMerge && probe.prNumber != null
                    ? t("settings.git.ghPROpen", { number: String(probe.prNumber), title: probe.prTitle ?? "" })
                    : probe && probe.ghAuthenticated
                      ? (() => {
                          const key = probeReasonKey(probe.reason);
                          return key ? t(key) : t("settings.git.ghPRReady");
                        })()
                      : t("settings.git.ghStatusUnknown")
                }
                action={
                  probe?.prUrl ? (
                    <button type="button" className="settings-gh-status__link" onClick={() => openExternal(probe.prUrl!)}>
                      {t("settings.git.ghOpenPR")}
                    </button>
                  ) : null
                }
              />
            </div>
            <div className="settings-block__footer settings-gh-block__footer">
              <SettingsActionButton primary={false} disabled={busy || probing} onClick={() => void runProbe()}>
                {probing ? t("settings.git.ghProbing") : t("settings.git.ghRunCheck")}
              </SettingsActionButton>
            </div>
          </>
        ) : null}
      </div>
    </SettingsBlock>
  );
}

function WorkspaceSection({
  s,
  busy,
  apply,
  ghCliSetupRequest,
  onGhCliSetupHandled,
}: SectionProps & {
  ghCliSetupRequest: (GitHubCliSettingsNavDetail & { id: number }) | null;
  onGhCliSetupHandled: () => void;
}) {
  const t = useT();
  const saved = normalizeDesktopGit(s.desktopGit);
  const [gitDraft, setGitDraft] = useState<DesktopGitView>(saved);
  const [commitInstructions, setCommitInstructions] = useState(saved.commitInstructions);
  const [prInstructions, setPrInstructions] = useState(saved.prInstructions);
  const [mergeSyncNote, setMergeSyncNote] = useState<string | null>(null);

  useEffect(() => {
    const next = normalizeDesktopGit(s.desktopGit);
    setGitDraft(next);
    setCommitInstructions(next.commitInstructions);
    setPrInstructions(next.prInstructions);
  }, [
    s.desktopGit?.prMergeMethod,
    s.desktopGit?.commitInstructions,
    s.desktopGit?.prInstructions,
    s.desktopGit?.checkGitHubCli,
    s.desktopGit?.syncRepoMergeToGitHub,
  ]);

  const persistGit = (next: DesktopGitView) => {
    const normalized = normalizeDesktopGit(next);
    setGitDraft(normalized);
    setCommitInstructions(normalized.commitInstructions);
    setPrInstructions(normalized.prInstructions);
    syncDesktopGitSettings(normalized);
    void apply(() => app.SetDesktopGitSettings(normalized));
  };

  const commitDirty = commitInstructions.trim() !== saved.commitInstructions;
  const prDirty = prInstructions.trim() !== saved.prInstructions;

  const saveCommitInstructions = () => {
    persistGit({ ...gitDraft, commitInstructions: commitInstructions.trim() });
  };

  const savePRInstructions = () => {
    persistGit({ ...gitDraft, prInstructions: prInstructions.trim() });
  };

  const setMergeMethod = (method: GitPRMergeMethod) => {
    if (method === gitDraft.prMergeMethod) return;
    persistGit({ ...gitDraft, prMergeMethod: method });
    if (!gitDraft.syncRepoMergeToGitHub) return;
    setMergeSyncNote(null);
    void syncGitHubRepoMergeMethod(method, (command) => app.RunShellQuiet(command)).then((result) => {
      setMergeSyncNote(
        result.ok ? t("settings.git.repoSyncOk", { repo: result.message }) : t("settings.git.repoSyncFailed"),
      );
    });
  };

  return (
    <>
      <GitHubCliSettingsBlock
        gitDraft={gitDraft}
        busy={busy}
        onChange={(next) => persistGit(next)}
        setupRequest={ghCliSetupRequest}
        onSetupHandled={onGhCliSetupHandled}
      />

      <SettingsBlock title={t("settings.git.prMergeTitle")} hint={t("settings.git.prMergeHint")}>
        <div className="set-seg set-seg--compact">
          {GIT_PR_MERGE_METHODS.map((method) => (
            <button
              key={method}
              type="button"
              className={`set-seg__btn${gitDraft.prMergeMethod === method ? " set-seg__btn--on" : ""}`}
              disabled={busy}
              onClick={() => setMergeMethod(method)}
            >
              {t(
                method === "merge"
                  ? "settings.git.mergeMethod.merge"
                  : method === "squash"
                    ? "settings.git.mergeMethod.squash"
                    : "settings.git.mergeMethod.rebase",
              )}
            </button>
          ))}
        </div>
        {mergeSyncNote && <p className="settings-block__note">{mergeSyncNote}</p>}
      </SettingsBlock>

      <SettingsBlock title={t("settings.git.commitInstructionsTitle")} hint={t("settings.git.commitInstructionsHint")}>
        <div className="settings-instructions-editor">
          <textarea
            className="settings-block__textarea mem-input"
            value={commitInstructions}
            disabled={busy}
            rows={4}
            placeholder={t("settings.git.commitInstructionsPlaceholder")}
            onChange={(event) => setCommitInstructions(event.target.value)}
          />
          <div className="settings-instructions-editor__bar">
            <SettingsSaveChip disabled={busy || !commitDirty} ready={commitDirty} onClick={saveCommitInstructions}>
              {t("common.save")}
            </SettingsSaveChip>
          </div>
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.git.prInstructionsTitle")} hint={t("settings.git.prInstructionsHint")}>
        <div className="settings-instructions-editor">
          <textarea
            className="settings-block__textarea mem-input"
            value={prInstructions}
            disabled={busy}
            rows={4}
            placeholder={t("settings.git.prInstructionsPlaceholder")}
            onChange={(event) => setPrInstructions(event.target.value)}
          />
          <div className="settings-instructions-editor__bar">
            <SettingsSaveChip disabled={busy || !prDirty} ready={prDirty} onClick={savePRInstructions}>
              {t("common.save")}
            </SettingsSaveChip>
          </div>
        </div>
      </SettingsBlock>
    </>
  );
}

function providerNames(providers: ProviderView[]): { value: string; label: string }[] {
  return [{ value: "", label: "—" }, ...providers.map((p) => ({ value: p.name, label: p.name }))];
}

const ARCDESK_COMPACT_DEFAULTS = {
  softCompactRatio: 0.5,
  compactRatio: 0.8,
  compactForceRatio: 0.9,
} as const;

const TEMPERATURE_OPTIONS = [0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1, 1.2, 1.5, 2];
const MAX_STEPS_OPTIONS = [0, 5, 10, 15, 20, 30, 50, 100, 150, 200];
const SOFT_COMPACT_OPTIONS = [0.45, 0.5, 0.55, 0.6];
const COMPACT_OPTIONS = [0.75, 0.8, 0.85, 0.9];
const FORCE_COMPACT_OPTIONS = [0.85, 0.9, 0.95];

function ensureNumericOption(options: number[], value: number): number[] {
  if (options.includes(value)) return options;
  return [...options, value].sort((a, b) => a - b);
}

function SettingsNumericSelect({
  value,
  onChange,
  options,
  disabled,
  className,
  formatLabel,
}: {
  value: number;
  onChange: (value: number) => void;
  options: number[];
  disabled?: boolean;
  className?: string;
  formatLabel: (value: number) => string;
}) {
  const merged = ensureNumericOption(options, value);
  return (
    <StudioSelect
      className={className ?? "set-grow"}
      value={String(value)}
      disabled={disabled}
      onChange={(next) => onChange(Number(next))}
      options={merged.map((option) => ({ value: String(option), label: formatLabel(option) }))}
    />
  );
}

function agentDraftFromSettings(s: SettingsView): AgentSettingsInput {
  const a = s.agent;
  const prompt = a.usesDefaultPrompt || !a.systemPrompt.trim() ? "" : a.systemPrompt;
  return {
    temperature: a.temperature,
    maxSteps: a.maxSteps,
    systemPrompt: prompt,
    systemPromptFile: a.systemPromptFile,
    outputStyle: a.outputStyle,
    autoPlan: a.autoPlan,
    autoPlanClassifier: a.autoPlanClassifier,
    softCompactRatio: a.softCompactRatio,
    compactRatio: a.compactRatio,
    compactForceRatio: a.compactForceRatio,
    subagentModel: a.subagentModel,
    subagentModels: { ...a.subagentModels },
  };
}

function agentDraftEquals(a: AgentSettingsInput, b: AgentSettingsInput): boolean {
  return JSON.stringify(a) === JSON.stringify(b);
}

function outputStyleLabel(name: string, t: ReturnType<typeof useT>): string {
  const key = `settings.agent.outputStyle.${name.replace(/-/g, "_")}` as const;
  const translated = t(key as "settings.agent.outputStyle.explanatory");
  return translated !== key ? translated : name;
}

function outputStyleDescription(name: string, styles: OutputStyleView[], t: ReturnType<typeof useT>): string {
  const key = `settings.agent.outputStyleDesc.${name.replace(/-/g, "_")}` as const;
  const translated = t(key as "settings.agent.outputStyleDesc.explanatory");
  if (translated !== key) return translated;
  return styles.find((st) => st.name === name)?.description ?? "";
}

function subagentSkillLabel(name: string, t: ReturnType<typeof useT>): string {
  const key = `settings.agent.skill.${name.replace(/-/g, "_")}` as const;
  const translated = t(key as "settings.agent.skill.explore");
  return translated !== key ? translated : name;
}

function normalizeAgentPromptForSave(draft: AgentSettingsInput, defaultPrompt: string): string {
  const trimmed = draft.systemPrompt.trim();
  const defaultTrimmed = defaultPrompt.trim();
  if (!trimmed || trimmed === defaultTrimmed) return "";
  return draft.systemPrompt;
}

function AgentSection({
  s,
  busy,
  apply,
  onNavigateTab,
}: SectionProps & { onNavigateTab: (tab: SettingsTab) => void }) {
  const t = useT();
  const saved = agentDraftFromSettings(s);
  const [draft, setDraft] = useState<AgentSettingsInput>(saved);
  const [outputStyles, setOutputStyles] = useState<OutputStyleView[]>([]);
  const [subagentSkills, setSubagentSkills] = useState<string[]>([]);
  const [advancedOpen, setAdvancedOpen] = useState(false);

  useEffect(() => {
    setDraft(agentDraftFromSettings(s));
  }, [s]);

  useEffect(() => {
    void app.ListOutputStyles().then(setOutputStyles).catch(() => setOutputStyles([]));
    void app
      .Capabilities()
      .then((view) => {
        const names = view.skills.filter((sk) => sk.runAs === "subagent").map((sk) => sk.name);
        setSubagentSkills(Array.from(new Set(names)).sort());
      })
      .catch(() => setSubagentSkills([]));
  }, []);

  const defaultPrompt = s.agent.defaultSystemPrompt;
  const dirty = !agentDraftEquals(
    { ...draft, systemPrompt: normalizeAgentPromptForSave(draft, defaultPrompt) },
    { ...saved, systemPrompt: normalizeAgentPromptForSave(saved, defaultPrompt) },
  );
  const providerOpts = providerNames(s.providers);
  const plannerLabel = s.plannerModel ? toRef(s.plannerModel, s) : t("common.none");
  const promptDisplay = draft.systemPrompt.trim() ? draft.systemPrompt : defaultPrompt;
  const promptDirty =
    normalizeAgentPromptForSave(draft, defaultPrompt) !== normalizeAgentPromptForSave(saved, defaultPrompt);

  const patchDraft = (patch: Partial<AgentSettingsInput>) => {
    setDraft((current) => ({ ...current, ...patch }));
  };

  const patchSubagentModel = (skill: string, model: string) => {
    setDraft((current) => {
      const next = { ...current.subagentModels };
      if (!model) delete next[skill];
      else next[skill] = model;
      return { ...current, subagentModels: next };
    });
  };

  const save = () => {
    const payload: AgentSettingsInput = {
      ...draft,
      systemPrompt: normalizeAgentPromptForSave(draft, defaultPrompt),
    };
    void apply(() => app.SetAgentSettings(payload));
  };

  const resetPrompt = () => patchDraft({ systemPrompt: "" });

  const resetCompactDefaults = () => {
    patchDraft({ ...ARCDESK_COMPACT_DEFAULTS });
  };

  const formatRatioLabel = (ratio: number) => `${Math.round(ratio * 100)}%`;

  const formatMaxStepsLabel = (steps: number) =>
    steps === 0 ? t("settings.agent.maxStepsUnlimited") : t("settings.agent.maxStepsValue", { n: String(steps) });

  const formatTemperatureLabel = (temp: number) => String(temp);

  const compactDefaultsDirty =
    draft.softCompactRatio !== ARCDESK_COMPACT_DEFAULTS.softCompactRatio ||
    draft.compactRatio !== ARCDESK_COMPACT_DEFAULTS.compactRatio ||
    draft.compactForceRatio !== ARCDESK_COMPACT_DEFAULTS.compactForceRatio;

  return (
    <>
      <SettingsBlock title={t("settings.agent.inferenceTitle")} hint={t("settings.agent.inferenceHint")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.agent.temperature")}</label>
            <SettingsNumericSelect
              value={draft.temperature}
              disabled={busy}
              options={TEMPERATURE_OPTIONS}
              formatLabel={formatTemperatureLabel}
              onChange={(value) => patchDraft({ temperature: value })}
            />
          </div>
          <div className="set-row">
            <label className="set-label">{t("settings.agent.maxSteps")}</label>
            <SettingsNumericSelect
              value={draft.maxSteps}
              disabled={busy}
              options={MAX_STEPS_OPTIONS}
              formatLabel={formatMaxStepsLabel}
              onChange={(value) => patchDraft({ maxSteps: value })}
            />
          </div>
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.promptTitle")} hint={t("settings.agent.systemPromptHint")}>
        <div className="settings-instructions-editor">
          <textarea
            id="agent-prompt"
            className="settings-block__textarea mem-input"
            rows={10}
            value={promptDisplay}
            disabled={busy}
            onChange={(e) => patchDraft({ systemPrompt: e.target.value })}
          />
          <div className="settings-instructions-editor__bar">
            <button type="button" className="settings-action-btn settings-action-btn--compact" disabled={busy || !promptDirty} onClick={resetPrompt}>
              {t("settings.agent.resetPrompt")}
            </button>
          </div>
        </div>
        <div className="set-row settings-agent-style-row">
          <label className="set-label" htmlFor="agent-style">
            {t("settings.agent.outputStyle")}
          </label>
          <StudioSelect
            className="set-grow"
            id="agent-style"
            value={draft.outputStyle}
            disabled={busy}
            onChange={(value) => patchDraft({ outputStyle: value })}
            options={[
              { value: "", label: t("settings.agent.outputStyleDefault") },
              ...outputStyles.map((style) => ({
                value: style.name,
                label: style.builtin
                  ? outputStyleLabel(style.name, t)
                  : `${style.name}（${t("settings.agent.outputStyleCustom")}）`,
              })),
            ]}
          />
        </div>
        {draft.outputStyle ? (
          <p className="settings-block__note settings-block__note--inline">
            {outputStyleDescription(draft.outputStyle, outputStyles, t)}
          </p>
        ) : null}
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.planningTitle")} hint={t("settings.agent.planningHint")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.autoPlan")}</label>
            <div className="set-seg set-seg--compact">
              {AUTO_PLAN_MODES.map((mode) => (
                <button
                  key={mode}
                  type="button"
                  className={`set-seg__btn${draft.autoPlan === mode ? " set-seg__btn--on" : ""}`}
                  disabled={busy}
                  onClick={() => patchDraft({ autoPlan: mode })}
                >
                  {mode === "on" ? t("settings.autoPlan.on") : t("settings.autoPlan.off")}
                </button>
              ))}
            </div>
          </div>
          <div className="set-row">
            <label className="set-label">{t("settings.plannerModel")}</label>
            <div className="settings-agent-linked-value">
              <span className="settings-agent-linked-value__text">{plannerLabel}</span>
              <button type="button" className="settings-action-btn settings-action-btn--compact" onClick={() => onNavigateTab("models")}>
                {t("settings.agent.openModels")}
              </button>
            </div>
          </div>
          <div className="set-row set-row--stack">
            <label className="set-label">{t("settings.agent.autoPlanClassifier")}</label>
            <StudioSelect
              className="set-grow"
              value={draft.autoPlanClassifier}
              disabled={busy || draft.autoPlan === "off"}
              onChange={(value) => patchDraft({ autoPlanClassifier: value })}
              options={providerOpts}
            />
            <p className="settings-block__note settings-block__note--inline">{t("settings.agent.autoPlanClassifierHint")}</p>
          </div>
          <p className="settings-block__note settings-block__note--inline">{t("settings.agent.permissionsNote")}</p>
          <button type="button" className="settings-action-btn settings-action-btn--compact" onClick={() => onNavigateTab("permissions")}>
            {t("settings.agent.openPermissions")}
          </button>
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.subagentTitle")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.agent.subagentDefault")}</label>
            <StudioSelect
              className="set-grow"
              value={draft.subagentModel}
              disabled={busy || providerOpts.length <= 1}
              onChange={(value) => patchDraft({ subagentModel: value })}
              options={providerOpts}
            />
          </div>
          {subagentSkills.length > 0 ? (
            <div className="settings-agent-subagent-list">
              {subagentSkills.map((skill) => (
                <div key={skill} className="set-row settings-agent-subagent-list__row">
                  <label className="set-label">{subagentSkillLabel(skill, t)}</label>
                  <StudioSelect
                    className="set-grow"
                    value={draft.subagentModels[skill] ?? ""}
                    disabled={busy || providerOpts.length <= 1}
                    onChange={(value) => patchSubagentModel(skill, value)}
                    options={providerOpts}
                  />
                </div>
              ))}
            </div>
          ) : (
            <p className="settings-block__note">{t("settings.agent.subagentEmpty")}</p>
          )}
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.agent.advancedTitle")} hint={t("settings.agent.advancedHint")}>
        <button
          type="button"
          className={`settings-agent-advanced-toggle${advancedOpen ? " settings-agent-advanced-toggle--open" : ""}`}
          aria-expanded={advancedOpen}
          onClick={() => setAdvancedOpen((open) => !open)}
        >
          <ChevronDown size={13} aria-hidden="true" />
          <span>{advancedOpen ? t("settings.agent.advancedHide") : t("settings.agent.advancedShow")}</span>
        </button>
        {advancedOpen ? (
          <div className="settings-block__form settings-agent-advanced-body">
            <div className="set-row set-row--stack">
              <label className="set-label">{t("settings.agent.systemPromptFile")}</label>
              <input
                id="agent-prompt-file"
                className="mem-input settings-block__input"
                value={draft.systemPromptFile}
                disabled={busy}
                placeholder={t("settings.agent.systemPromptFilePlaceholder")}
                onChange={(e) => patchDraft({ systemPromptFile: e.target.value })}
              />
              <p className="settings-block__note settings-block__note--inline">{t("settings.agent.systemPromptFileHint")}</p>
            </div>
            <div className="set-row">
              <label className="set-label">{t("settings.agent.softCompactRatio")}</label>
              <SettingsNumericSelect
                value={draft.softCompactRatio}
                disabled={busy}
                options={SOFT_COMPACT_OPTIONS}
                formatLabel={formatRatioLabel}
                onChange={(value) => patchDraft({ softCompactRatio: value })}
              />
            </div>
            <div className="set-row">
              <label className="set-label">{t("settings.agent.compactRatio")}</label>
              <SettingsNumericSelect
                value={draft.compactRatio}
                disabled={busy}
                options={COMPACT_OPTIONS}
                formatLabel={formatRatioLabel}
                onChange={(value) => patchDraft({ compactRatio: value })}
              />
            </div>
            <div className="set-row">
              <label className="set-label">{t("settings.agent.compactForceRatio")}</label>
              <SettingsNumericSelect
                value={draft.compactForceRatio}
                disabled={busy}
                options={FORCE_COMPACT_OPTIONS}
                formatLabel={formatRatioLabel}
                onChange={(value) => patchDraft({ compactForceRatio: value })}
              />
            </div>
            <p className="settings-block__note settings-block__note--inline">{t("settings.agent.compactHint")}</p>
            <div className="settings-agent-prompt-actions">
              <button
                type="button"
                className="settings-action-btn settings-action-btn--compact"
                disabled={busy || !compactDefaultsDirty}
                onClick={resetCompactDefaults}
              >
                {t("settings.agent.resetCompactDefaults")}
              </button>
            </div>
          </div>
        ) : null}
      </SettingsBlock>

      <div className="settings-agent-save-row">
        <SettingsSaveChip disabled={busy || !dirty} ready={dirty} onClick={save}>
          {t("settings.agent.save")}
        </SettingsSaveChip>
      </div>
    </>
  );
}

function ModelsServicesSection({ s, busy, apply }: SectionProps) {
  const t = useT();
  const deepseek = deepseekProviders(s.providers)[0] ?? s.providers[0];
  const [fetchedModels, setFetchedModels] = useState<string[] | null>(null);
  const [modelsExpanded, setModelsExpanded] = useState(false);
  const [fetchCount, setFetchCount] = useState<number | null>(null);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [fetching, setFetching] = useState(false);
  const modelsToggleRef = useRef<HTMLButtonElement>(null);
  const modelsPanelRef = useRef<HTMLDivElement>(null);

  useDismissOnOutsidePointerDown(modelsExpanded, () => setModelsExpanded(false), {
    excludeRefs: [modelsToggleRef, modelsPanelRef],
  });

  const modelRefs =
    fetchedModels && deepseek ? fetchedModels.map((model) => `${deepseek.name}/${model}`) : [];
  const modelsReady = modelRefs.length > 0;
  const selectPlaceholder = t("settings.models.selectAfterFetch");

  const syncModels = () => {
    if (!deepseek) return;
    setFetching(true);
    setFetchCount(null);
    setFetchError(null);
    void apply(async () => {
      const result = await app.SyncProviderModels(deepseek.name);
      setFetchedModels(result.models);
      setFetchCount(result.models.length);
    })
      .catch((e) => {
        setFetchError(String((e as Error)?.message ?? e));
      })
      .finally(() => {
        setFetching(false);
      });
  };

  const defaultRef = (() => {
    if (!modelsReady) return "";
    const ref = toRef(s.defaultModel, s);
    return modelRefs.includes(ref) ? ref : modelRefs[0];
  })();

  const plannerRef = (() => {
    if (!modelsReady) return "";
    if (!s.plannerModel) return "";
    const ref = toRef(s.plannerModel, s);
    return modelRefs.includes(ref) ? ref : "";
  })();

  return (
    <SettingsBlock title={t("settings.providers")}>
      <div className="settings-block__form">
        <div className="set-row set-row--stack">
          <label className="set-label">{t("settings.models.baseUrlTitle")}</label>
          <DeepSeekServiceUrlField providers={s.providers} busy={busy} apply={apply} />
          <p className="settings-block__note settings-block__note--inline">{t("settings.models.baseUrlHint")}</p>
        </div>

        <div className="set-row set-row--stack">
          <label className="set-label">{t("settings.models.apiTitle")}</label>
          {deepseek?.keySet ? (
            <span className="settings-block__status">{t("settings.general.keyConfigured")}</span>
          ) : (
            <span className="settings-block__status settings-block__status--warn">{t("settings.general.keyMissing")}</span>
          )}
          {deepseek ? (
            <KeyField apiKeyEnv={deepseek.apiKeyEnv} busy={busy} onSet={(v) => apply(() => app.SetProviderKey(deepseek.apiKeyEnv, v))} />
          ) : (
            <p className="settings-block__note">{t("settings.models.apiMissingProvider")}</p>
          )}
          <p className="settings-block__note settings-block__note--inline">{t("settings.models.apiHint")}</p>
        </div>

        <div className="set-row set-row--stack">
          <label className="set-label">{t("settings.models.fetchTitle")}</label>
          <div className="settings-models-fetch-row">
            <button
              type="button"
              ref={modelsToggleRef}
              className={`settings-models-list-toggle${modelsExpanded ? " settings-models-list-toggle--open" : ""}`}
              disabled={!fetchedModels?.length}
              aria-expanded={modelsExpanded}
              onClick={() => setModelsExpanded((open) => !open)}
            >
              <ChevronDown size={13} className="settings-models-list-toggle__caret" aria-hidden="true" />
              <span>
                {fetchedModels?.length
                  ? t("settings.models.listToggle", { count: String(fetchedModels.length) })
                  : t("settings.models.listToggleEmpty")}
              </span>
            </button>
            <button
              type="button"
              className={`settings-action-btn settings-models-fetch-btn${fetching ? " settings-models-fetch-btn--loading" : ""}`}
              disabled={busy || fetching || !deepseek?.keySet}
              onClick={syncModels}
            >
              {fetching ? (
                <Loader2 size={14} className="dock-panel__spin" aria-hidden="true" />
              ) : (
                t("settings.models.fetchModels")
              )}
            </button>
          </div>
          {modelsExpanded && fetchedModels && fetchedModels.length > 0 ? (
            <div ref={modelsPanelRef} className="settings-models-list-panel" role="list">
              {fetchedModels.map((model) => (
                <span key={model} className="settings-models-list-panel__item" role="listitem">
                  {model}
                </span>
              ))}
            </div>
          ) : null}
          {fetchCount !== null ? (
            <p className="settings-block__note settings-block__note--inline settings-models-fetch-note--ok">
              {t("settings.models.fetchOk", { count: String(fetchCount) })}
            </p>
          ) : null}
          {fetchError ? <p className="settings-block__note settings-block__note--inline">{fetchError}</p> : null}
        </div>

        <div className="set-row">
          <label className="set-label">{t("settings.defaultModel")}</label>
          <StudioSelect
            className="set-grow"
            value={defaultRef}
            disabled={busy || !modelsReady}
            placeholder={selectPlaceholder}
            onChange={(value) => void apply(() => app.SetDefaultModel(value))}
            options={modelRefs.map((ref) => ({ value: ref, label: ref }))}
          />
        </div>

        <div className="set-row">
          <label className="set-label">{t("settings.plannerModel")}</label>
          <StudioSelect
            className="set-grow"
            value={plannerRef}
            disabled={busy || !modelsReady}
            placeholder={selectPlaceholder}
            onChange={(value) => void apply(() => app.SetPlannerModel(value))}
            options={[
              { value: "", label: t("settings.plannerNone") },
              ...modelRefs.map((ref) => ({ value: ref, label: ref })),
            ]}
          />
        </div>
      </div>
    </SettingsBlock>
  );
}

function KeyField({ apiKeyEnv, busy, onSet }: { apiKeyEnv: string; busy: boolean; onSet: (v: string) => Promise<void> }) {
  const t = useT();
  const [val, setVal] = useState("");
  if (!apiKeyEnv) return null;
  return (
    <div className="set-key">
      <input
        className="mem-input"
        type="password"
        placeholder={t("settings.setKey", { env: apiKeyEnv })}
        value={val}
        onChange={(e) => setVal(e.target.value)}
      />
      <SettingsSaveChip
        disabled={busy || !val.trim()}
        ready={!!val.trim()}
        onClick={() => {
          void onSet(val.trim());
          setVal("");
        }}
      >
        {t("settings.saveKey")}
      </SettingsSaveChip>
    </div>
  );
}

const PERMISSION_RULE_LISTS = ["deny", "ask", "allow"] as const;

function PermissionsSection({ s, busy, apply }: SectionProps) {
  const t = useT();
  const perms = s.permissions;
  const sb = s.sandbox;
  const [root, setRoot] = useState(sb.workspaceRoot);
  const [previewHosts, setPreviewHosts] = useState<string[]>([]);
  const [previewPorts, setPreviewPorts] = useState<number[]>([]);
  const [previewStrict, setPreviewStrict] = useState(false);
  const [savedPreview, setSavedPreview] = useState({ hosts: [] as string[], ports: [] as number[], strict: false });

  useEffect(() => {
    setRoot(sb.workspaceRoot);
  }, [sb.workspaceRoot]);

  useEffect(() => {
    let cancelled = false;
    void app
      .ProjectSandboxStatus()
      .then((status) => {
        if (cancelled) return;
        const hosts = status.previewHosts ?? [];
        const ports = status.previewPorts ?? [];
        const strict = status.previewStrict ?? false;
        setPreviewHosts(hosts);
        setPreviewPorts(ports);
        setPreviewStrict(strict);
        setSavedPreview({ hosts, ports, strict });
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [s.configPath]);

  const setSandbox = (next: Partial<typeof sb>) =>
    apply(() =>
      app.SetSandbox(
        next.bash ?? sb.bash,
        next.network ?? sb.network,
        next.workspaceRoot ?? sb.workspaceRoot,
        next.allowWrite ?? sb.allowWrite,
      ),
    );

  const previewDirty =
    previewStrict !== savedPreview.strict ||
    previewHosts.join("\0") !== savedPreview.hosts.join("\0") ||
    previewPorts.join(",") !== savedPreview.ports.join(",");

  const savePreview = () => {
    void apply(async () => {
      await app.SaveProjectPreviewSettings({
        previewHosts,
        previewPorts,
        previewStrict,
      });
      setSavedPreview({ hosts: [...previewHosts], ports: [...previewPorts], strict: previewStrict });
    });
  };

  return (
    <>
      <SettingsBlock title={t("settings.permissions.writerModeTitle")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.permissions.writerFallback")}</label>
            <StudioSelect
              className="set-grow"
              value={perms.mode}
              disabled={busy}
              onChange={(value) => void apply(() => app.SetPermissionMode(value))}
              options={[
                { value: "ask", label: t("settings.modeAsk") },
                { value: "allow", label: t("settings.modeAllow") },
                { value: "deny", label: t("settings.modeDeny") },
              ]}
            />
          </div>
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.permissions.rulesTitle")}>
        <div className="settings-block__form">
          {PERMISSION_RULE_LISTS.map((list) => (
            <RuleList
              key={list}
              collapsible
              list={list}
              rules={perms[list]}
              busy={busy}
              title={t(`settings.permissions.list.${list}`)}
              tone={list}
              placeholder={t("settings.permissions.addRulePlaceholder")}
              onAdd={(rule) => apply(() => app.AddPermissionRule(list, rule))}
              onRemove={(rule) => apply(() => app.RemovePermissionRule(list, rule))}
            />
          ))}
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.sandboxTitle")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.bashSandbox")}</label>
            <StudioSelect
              className="set-grow"
              value={sb.bash}
              disabled={busy}
              onChange={(value) => void setSandbox({ bash: value })}
              options={[
                { value: "enforce", label: t("settings.bashEnforce") },
                { value: "off", label: t("settings.bashOff") },
              ]}
            />
          </div>
          <label className="set-check">
            <input type="checkbox" checked={sb.network} disabled={busy} onChange={(e) => void setSandbox({ network: e.target.checked })} />
            {t("settings.allowNetwork")}
          </label>
          <div className="set-row">
            <label className="set-label">{t("settings.workspaceRoot")}</label>
            <input
              className="mem-input set-grow"
              placeholder={t("settings.workspaceDefault")}
              value={root}
              disabled={busy}
              onChange={(e) => setRoot(e.target.value)}
              onBlur={() => root !== sb.workspaceRoot && void setSandbox({ workspaceRoot: root })}
            />
          </div>
          <RuleList
            collapsible
            list="allow_write"
            rules={sb.allowWrite}
            busy={busy}
            title={t("settings.permissions.allowWrite")}
            placeholder={t("settings.permissions.allowWritePlaceholder")}
            onAdd={(dir) => setSandbox({ allowWrite: [...sb.allowWrite, dir] })}
            onRemove={(dir) => setSandbox({ allowWrite: sb.allowWrite.filter((x) => x !== dir) })}
          />
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.permissions.previewTitle")}>
        <div className="settings-block__form">
          <ProjectPreviewSettings
            busy={busy}
            previewStrict={previewStrict}
            onPreviewStrictChange={setPreviewStrict}
            previewHosts={previewHosts}
            onPreviewHostsChange={setPreviewHosts}
            previewPorts={previewPorts}
            onPreviewPortsChange={setPreviewPorts}
          />
          <div className="settings-permissions-preview-save">
            <SettingsSaveChip disabled={busy || !previewDirty} ready={previewDirty} onClick={savePreview}>
              {t("settings.permissions.savePreview")}
            </SettingsSaveChip>
          </div>
        </div>
      </SettingsBlock>
    </>
  );
}

function AppearanceSection({
  theme,
  themeStyle,
  backgroundPreset,
  foregroundPreset,
  textSize,
  codeFontSize,
  diffMarker,
  onTheme,
  onThemeStyle,
  onBackgroundPreset,
  onForegroundPreset,
  onTextSize,
  onCodeFontSize,
  onDiffMarker,
}: {
  theme: Theme;
  themeStyle: ThemeStyle;
  backgroundPreset: BackgroundPreset;
  foregroundPreset: ForegroundPreset;
  textSize: TextSize;
  codeFontSize: CodeFontSize;
  diffMarker: DiffMarkerStyle;
  onTheme: (t: Theme) => void;
  onThemeStyle: (style: ThemeStyle) => void;
  onBackgroundPreset: (preset: BackgroundPreset) => void;
  onForegroundPreset: (preset: ForegroundPreset) => void;
  onTextSize: (size: TextSize) => void;
  onCodeFontSize: (size: CodeFontSize) => void;
  onDiffMarker: (style: DiffMarkerStyle) => void;
}) {
  const t = useT();
  const themeOptions: Theme[] = ["auto", "light", "dark"];
  const accentOptions = stylesForTheme(theme);
  const resolvedTheme = getResolvedTheme(theme);
  const backgroundIds = backgroundPresetsForTheme(resolvedTheme);
  const safeBackground = backgroundIds.includes(backgroundPreset) ? backgroundPreset : backgroundIds[0]!;
  const foregroundIds = foregroundPresetsForBackground(safeBackground);
  const safeForeground = foregroundIds.includes(foregroundPreset) ? foregroundPreset : foregroundIds[0]!;
  const backgroundOptions = backgroundIds.map((id) => ({
    value: id,
    label: backgroundPresetName(id, t),
    icon: colorSwatch(backgroundColorFor(id)),
  }));
  const foregroundOptions = foregroundIds.map((id) => ({
    value: id,
    label: foregroundPresetName(id, t),
    icon: colorSwatch(foregroundColorFor(id)),
  }));

  return (
    <>
      <SettingsBlock title={t("settings.theme")}>
        <div className="set-seg set-seg--compact">
          {themeOptions.map((opt) => (
            <button
              key={opt}
              type="button"
              className={`set-seg__btn${theme === opt ? " set-seg__btn--on" : ""}`}
              onClick={() => onTheme(opt)}
            >
              {themeName(opt, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
      <SettingsBlock title={t("settings.themeStyle")}>
        <div className="settings-accent-grid">
          {accentOptions.map((style) => (
            <button
              key={style}
              type="button"
              className={`settings-accent-swatch${themeStyle === style ? " settings-accent-swatch--on" : ""}`}
              data-style={style}
              onClick={() => onThemeStyle(style)}
            >
              <span className="settings-accent-swatch__dot" />
              {themeStyleName(style, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
      <SettingsBlock title={t("settings.backgroundColor")}>
        <StudioSelect
          value={safeBackground}
          onChange={(value) => onBackgroundPreset(value as BackgroundPreset)}
          options={backgroundOptions}
        />
      </SettingsBlock>
      <SettingsBlock title={t("settings.foregroundColor")}>
        <StudioSelect
          value={safeForeground}
          onChange={(value) => onForegroundPreset(value as ForegroundPreset)}
          options={foregroundOptions}
        />
      </SettingsBlock>
      <SettingsBlock title={t("settings.uiFontSize")}>
        <div className="set-seg set-seg--compact">
          {TEXT_SIZES.map((size) => (
            <button
              key={size}
              type="button"
              className={`set-seg__btn${textSize === size ? " set-seg__btn--on" : ""}`}
              onClick={() => onTextSize(size)}
            >
              {textSizeName(size, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
      <SettingsBlock title={t("settings.codeFontSize")}>
        <div className="set-seg set-seg--compact">
          {CODE_FONT_SIZES.map((size) => (
            <button
              key={size}
              type="button"
              className={`set-seg__btn${codeFontSize === size ? " set-seg__btn--on" : ""}`}
              onClick={() => onCodeFontSize(size)}
            >
              {textSizeName(size, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
      <SettingsBlock title={t("settings.diffMarker")}>
        <p className="settings-block__card-lead">{t("settings.diffMarkerHint")}</p>
        <div className="set-seg set-seg--compact">
          {DIFF_MARKER_STYLES.map((style) => (
            <button
              key={style}
              type="button"
              className={`set-seg__btn${diffMarker === style ? " set-seg__btn--on" : ""}`}
              onClick={() => onDiffMarker(style)}
            >
              {diffMarkerName(style, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
    </>
  );
}

function colorSwatch(hex: string) {
  return <span className="settings-color-swatch" style={{ background: hex }} aria-hidden />;
}

function backgroundPresetName(id: BackgroundPreset, t: ReturnType<typeof useT>): string {
  return t(`settings.bg.${id}`);
}

function foregroundPresetName(id: ForegroundPreset, t: ReturnType<typeof useT>): string {
  return t(`settings.fg.${id}`);
}

function diffMarkerName(style: DiffMarkerStyle, t: ReturnType<typeof useT>): string {
  switch (style) {
    case "background":
      return t("settings.diffMarkerBackground");
    case "signs":
      return t("settings.diffMarkerSigns");
  }
}

function themeStyleName(style: ThemeStyle, t: ReturnType<typeof useT>): string {
  switch (style) {
    case "graphite":
      return t("settings.style.graphite");
    case "ember":
      return t("settings.style.ember");
    case "aurora":
      return t("settings.style.aurora");
    case "midnight":
      return t("settings.style.midnight");
    case "cobalt":
      return t("settings.style.cobalt");
    case "sandstone":
      return t("settings.style.sandstone");
    case "porcelain":
      return t("settings.style.porcelain");
    case "linen":
      return t("settings.style.linen");
    case "glacier":
      return t("settings.style.glacier");
  }
}

function themeName(theme: Theme, t: ReturnType<typeof useT>): string {
  switch (theme) {
    case "auto":
      return t("settings.themeAuto");
    case "light":
      return t("settings.themeLight");
    case "dark":
      return t("settings.themeDark");
  }
}

function textSizeName(size: TextSize, t: ReturnType<typeof useT>): string {
  switch (size) {
    case "small":
      return t("settings.textSizeSmall");
    case "default":
      return t("settings.textSizeDefault");
    case "large":
      return t("settings.textSizeLarge");
    case "xlarge":
      return t("settings.textSizeXLarge");
  }
}

const MB = 1024 * 1024;
const mb = (n: number) => (n / MB).toFixed(1);

// UpdatesSection is the manual side of the auto-updater: it shows the running
// version and a Check button, then the same state machine the top banner uses
// (useUpdater) — available → install/download, with progress and errors inline.
function UpdatesSection({ configPath }: { configPath: string }) {
  const t = useT();
  const { status, check, apply } = useUpdater();
  const [version, setVersion] = useState("");
  useEffect(() => {
    app.Version().then(setVersion).catch(() => {});
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
