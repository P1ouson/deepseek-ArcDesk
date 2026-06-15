import { useCallback, useEffect, useState, type ReactNode } from "react";
import type { GitHubCliSettingsNavDetail } from "../../lib/gitHubCliSettingsNav";
import { app, openExternal } from "../../lib/bridge";
import { useT } from "../../lib/i18n";
import { normalizeDesktopGit, syncDesktopGitSettings } from "../../lib/desktopGitPrefs";
import {
  GITHUB_CLI_INSTALL_URL,
  installGitHubCliViaApp,
  probeGitHubCli,
  probeReasonKey,
  syncGitHubRepoMergeMethod,
  type GitHubCliProbe,
} from "../../lib/gitHubCli";
import type { DesktopGitView, GitPRMergeMethod } from "../../lib/types";
import type { RightDockTab } from "../Topbar";
import {
  SettingsActionButton,
  SettingsBlock,
  SettingsSaveChip,
  type SettingsSectionProps,
} from "../settingsPrimitives";
import { SettingsShortcutRow } from "./SettingsShortcutRow";

const GIT_PR_MERGE_METHODS: GitPRMergeMethod[] = ["merge", "squash", "rebase"];

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
  const [ghInstalling, setGhInstalling] = useState(false);
  const [ghInstallNotice, setGhInstallNotice] = useState<string | null>(null);

  const runProbe = useCallback(async (): Promise<GitHubCliProbe | null> => {
    setProbing(true);
    try {
      const result = await probeGitHubCli((command) => app.RunShellQuiet(command));
      setProbe(result);
      return result;
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

  const handleInstallGh = useCallback(async () => {
    setGhInstalling(true);
    setGhInstallNotice(null);
    try {
      const result = await installGitHubCliViaApp(() => app.InstallGitHubCLI());
      const nextProbe = await runProbe();
      if (result.ok) {
        if (nextProbe?.ghInstalled) {
          return;
        }
        setGhInstallNotice(t("git.ghInstallNeedRestart"));
        return;
      }
      setGhInstallNotice(t("git.ghInstallFailed"));
      openExternal(GITHUB_CLI_INSTALL_URL);
    } catch {
      setGhInstallNotice(t("git.ghInstallFailed"));
      openExternal(GITHUB_CLI_INSTALL_URL);
    } finally {
      setGhInstalling(false);
    }
  }, [runProbe, t]);

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
                    <button
                      type="button"
                      className="settings-gh-status__link"
                      onClick={() => void handleInstallGh()}
                      disabled={ghInstalling || probing}
                    >
                      {ghInstalling ? t("git.ghInstalling") : t("settings.git.ghInstallLink")}
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
            {ghInstallNotice ? <p className="settings-gh-status__notice">{ghInstallNotice}</p> : null}
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

export function WorkspaceSection({
  s,
  busy,
  apply,
  ghCliSetupRequest,
  onGhCliSetupHandled,
  onOpenHistory,
  onOpenMemory,
  onOpenTrash,
  onOpenDockTab,
  onOpenTerminal,
}: SettingsSectionProps & {
  ghCliSetupRequest: (GitHubCliSettingsNavDetail & { id: number }) | null;
  onGhCliSetupHandled: () => void;
  onOpenHistory?: () => void;
  onOpenMemory?: () => void;
  onOpenTrash?: () => void;
  onOpenDockTab?: (tab: RightDockTab) => void;
  onOpenTerminal?: () => void;
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
      <SettingsBlock title={t("settings.workspace.devToolsTitle")} hint={t("settings.workspace.devToolsHint")}>
        <div className="settings-block__stack settings-block__stack--shortcut-actions">
          <SettingsShortcutRow
            title={t("settings.workspace.openFiles")}
            hint={t("settings.workspace.openFilesDesc")}
            buttonLabel={t("settings.workspace.openFiles")}
            onClick={onOpenDockTab ? () => onOpenDockTab("files") : undefined}
            disabled={busy}
          />
          <SettingsShortcutRow
            title={t("settings.workspace.openChanges")}
            hint={t("settings.workspace.openChangesDesc")}
            buttonLabel={t("settings.workspace.openChanges")}
            onClick={onOpenDockTab ? () => onOpenDockTab("changes") : undefined}
            disabled={busy}
          />
          <SettingsShortcutRow
            title={t("settings.workspace.openGit")}
            hint={t("settings.workspace.openGitDesc")}
            buttonLabel={t("settings.workspace.openGit")}
            onClick={onOpenDockTab ? () => onOpenDockTab("git") : undefined}
            disabled={busy}
          />
          <SettingsShortcutRow
            title={t("settings.workspace.openTerminal")}
            hint={t("settings.workspace.openTerminalDesc")}
            buttonLabel={t("settings.workspace.openTerminal")}
            onClick={onOpenTerminal}
            disabled={busy}
          />
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.workspace.dataTitle")} hint={t("settings.workspace.dataHint")}>
        <div className="settings-block__stack settings-block__stack--shortcut-actions">
          <SettingsShortcutRow
            title={t("settings.workspace.openHistory")}
            hint={t("settings.workspace.openHistoryDesc")}
            buttonLabel={t("settings.workspace.openHistory")}
            onClick={onOpenHistory}
            disabled={busy}
          />
          <SettingsShortcutRow
            title={t("settings.workspace.openMemory")}
            hint={t("settings.workspace.openMemoryDesc")}
            buttonLabel={t("settings.workspace.openMemory")}
            onClick={onOpenMemory}
            disabled={busy}
          />
          <SettingsShortcutRow
            title={t("settings.workspace.openTrash")}
            hint={t("settings.workspace.openTrashDesc")}
            buttonLabel={t("settings.workspace.openTrash")}
            onClick={onOpenTrash}
            disabled={busy}
          />
        </div>
      </SettingsBlock>

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
