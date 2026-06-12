import { lazy, Suspense, useEffect, useRef, useState } from "react";
import {
  GITHUB_CLI_SETTINGS_EVENT,
  type GitHubCliSettingsNavDetail,
} from "../lib/gitHubCliSettingsNav";
import {
  Bot,
  BarChart3,
  Cpu,
  FolderGit2,
  Palette,
  RefreshCw,
  Settings2,
  Shield,
} from "lucide-react";
import { app } from "../lib/bridge";
import { useAsyncMutation } from "../lib/useAsyncMutation";
import { applyThemeFromSettings } from "../lib/applyThemeFromSettings";
import { useT } from "../lib/i18n";
import type { AppMode } from "../lib/appMode";
import type { RightDockTab } from "./Topbar";
import type { SettingsView } from "../lib/types";
import { UsageInsightsSettings } from "./UsageInsightsSettings";
import { SettingsPageShell } from "./settingsPrimitives";
import { normalizeSettingsView } from "./settings/normalizeSettingsView";
import { SETTINGS_TABS, type SettingsTab } from "./settings/types";
import { useAppearanceSettingsState } from "./settings/useAppearanceSettingsState";

export { SettingsBlock, SettingsSaveChip, SettingsActionButton } from "./settingsPrimitives";

const GeneralSection = lazy(() =>
  import("./settings/GeneralSection").then((m) => ({ default: m.GeneralSection })),
);
const AppearanceSection = lazy(() =>
  import("./settings/AppearanceSection").then((m) => ({ default: m.AppearanceSection })),
);
const WorkspaceSection = lazy(() =>
  import("./settings/WorkspaceSection").then((m) => ({ default: m.WorkspaceSection })),
);
const ModelsServicesSection = lazy(() =>
  import("./settings/ModelsServicesSection").then((m) => ({ default: m.ModelsServicesSection })),
);
const AgentSection = lazy(() =>
  import("./settings/AgentSection").then((m) => ({ default: m.AgentSection })),
);
const PermissionsSection = lazy(() =>
  import("./settings/PermissionsSection").then((m) => ({ default: m.PermissionsSection })),
);
const UpdatesSection = lazy(() =>
  import("./settings/UpdatesSection").then((m) => ({ default: m.UpdatesSection })),
);

export interface SettingsPageProps {
  onChanged: () => void;
  onComposerPrompt?: (text: string) => void;
  onModeChange?: (mode: AppMode) => void;
  onOpenHistory?: () => void;
  onOpenMemory?: () => void;
  onOpenCapabilities?: () => void;
  onOpenTrash?: () => void;
  onConfigureProjectSandbox?: () => void;
  onOpenDockTab?: (tab: RightDockTab) => void;
  onOpenTerminal?: () => void;
  onOpenOnboarding?: () => void;
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

// SettingsPage is the full-page settings surface (Codex / Claude Code style).
export function SettingsPage({
  onChanged,
  onComposerPrompt,
  onOpenHistory,
  onOpenMemory,
  onOpenCapabilities,
  onOpenTrash,
  onOpenDockTab,
  onOpenTerminal,
  onModeChange,
  onOpenOnboarding,
}: SettingsPageProps) {
  const t = useT();
  const [s, setS] = useState<SettingsView | null>(null);
  const { busy, err, run: runMutation } = useAsyncMutation();
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

  const apply = async (fn: () => Promise<void>) => {
    await runMutation(async () => {
      await fn();
      const latest = normalizeSettingsView(await app.Settings());
      setS(latest);
      if (latest) applyThemeFromSettings(latest, "settings");
      onChanged();
    });
  };

  const appearance = useAppearanceSettingsState(s, apply);

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
                <Suspense fallback={<div className="empty settings-studio__loading">{t("settings.loading")}</div>}>
                  {tab === "general" && (
                    <>
                      <GeneralSection
                        s={s}
                        busy={busy}
                        apply={apply}
                        onModeChange={onModeChange}
                        onOpenCapabilities={onOpenCapabilities}
                        onOpenOnboarding={onOpenOnboarding}
                      />
                    </>
                  )}
                  {tab === "appearance" && <AppearanceSection {...appearance} />}
                  {tab === "workspace" && (
                    <WorkspaceSection
                      s={s}
                      busy={busy}
                      apply={apply}
                      ghCliSetupRequest={ghCliSetupRequest}
                      onGhCliSetupHandled={() => setGhCliSetupRequest(null)}
                      onOpenHistory={onOpenHistory}
                      onOpenMemory={onOpenMemory}
                      onOpenTrash={onOpenTrash}
                      onOpenDockTab={onOpenDockTab}
                      onOpenTerminal={onOpenTerminal}
                    />
                  )}
                  {tab === "models" && <ModelsServicesSection s={s} busy={busy} apply={apply} />}
                  {tab === "usage" && <UsageInsightsSettings />}
                  {tab === "agent" && (
                    <AgentSection
                      s={s}
                      busy={busy}
                      apply={apply}
                      onNavigateTab={setTab}
                      onComposerPrompt={onComposerPrompt}
                    />
                  )}
                  {tab === "permissions" && <PermissionsSection s={s} busy={busy} apply={apply} />}
                  {tab === "updates" && <UpdatesSection configPath={s.configPath} />}
                </Suspense>
              </SettingsPageShell>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
