import { useCallback, useEffect, useMemo, useState } from "react";
import type { MouseEvent as ReactMouseEvent } from "react";
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Check,
  GitCommitHorizontal,
  Loader2,
  MessageSquarePlus,
  Plus,
  RefreshCw,
  RotateCcw,
  Sparkles,
  Undo2,
  X,
} from "lucide-react";
import { app } from "../lib/bridge";
import { isGitHubCliCheckEnabled } from "../lib/desktopGitPrefs";
import { openGitHubCliSettings } from "../lib/gitHubCliSettingsNav";
import { GitHubCliSetupModal, type GitHubCliSetupReason } from "./GitHubCliSetupModal";
import { useT } from "../lib/i18n";
import { shellQuote } from "../lib/shellQuote";
import { useDismissOnClickOutside } from "../lib/useDismissOnClickOutside";
import { useWorkspaceChanges } from "../lib/useWorkspaceChanges";
import type { WorkspaceChangeView } from "../lib/types";
import { buildCommitMessagePrompt, buildPRPrompt, ghPRMergeCommand } from "../lib/gitPrompts";
import { probeGitHubCli, probeReasonKey, type GitHubCliProbe } from "../lib/gitHubCli";
import { addWorkspaceFileContentToChat } from "../lib/workspaceAddToChat";
import { basename, shortCwd } from "../lib/workspaceFilePreview";
import { formatWorkspaceReference } from "../lib/workspaceDrag";
import { FloatingMenu, FloatingMenuItems } from "./FloatingMenu";
import { Tooltip } from "./Tooltip";

interface GitPanelProps {
  cwd?: string;
  refreshKey?: number;
  activeFilePath?: string | null;
  onOpenFile?: (path: string) => void;
  onAddToChat?: (text: string) => void;
}

interface GitCommandStatus {
  command: string;
  output: string;
  err?: string;
}

type GitStatusTone = "mod" | "add" | "del" | "unk";

function hasGit(row: WorkspaceChangeView): boolean {
  return row.sources.includes("git");
}

function isDeletedChange(row: WorkspaceChangeView): boolean {
  return !!row.gitStatus && row.gitStatus.includes("D");
}

function sortGitRows(rows: WorkspaceChangeView[]): WorkspaceChangeView[] {
  return [...rows].sort((a, b) => a.path.localeCompare(b.path));
}

function gitStatusTone(status: string): GitStatusTone {
  const s = status.trim().toUpperCase();
  if (s.includes("D")) return "del";
  if (s.includes("?")) return "unk";
  if (s.includes("A")) return "add";
  return "mod";
}

function formatGitRowsForPrompt(rows: WorkspaceChangeView[], max = 12): string {
  const list = rows.slice(0, max).map((row) => `- ${row.path}${row.gitStatus ? ` (${row.gitStatus})` : ""}`).join("\n");
  const more = rows.length > max ? `\n… +${rows.length - max} more` : "";
  return `${list}${more}`;
}

function summarizeOutput(text: string, max = 96): string {
  const line = text.trim().split("\n").find((part) => part.trim()) ?? "";
  if (line.length <= max) return line;
  return `${line.slice(0, max - 1)}…`;
}

const ROW_MENU_HEIGHT = 200;

export function GitPanel({ cwd, refreshKey, activeFilePath, onOpenFile, onAddToChat }: GitPanelProps) {
  const t = useT();
  const { changes, loading, loadChanges } = useWorkspaceChanges(cwd, refreshKey);
  const [query, setQuery] = useState("");
  const [commitMessage, setCommitMessage] = useState("");
  const [gitBusy, setGitBusy] = useState(false);
  const [lastStatus, setLastStatus] = useState<GitCommandStatus | null>(null);
  const [rowMenu, setRowMenu] = useState<{ x: number; y: number; row: WorkspaceChangeView } | null>(null);
  const [ghProbe, setGhProbe] = useState<GitHubCliProbe | null>(null);
  const [ghProbing, setGhProbing] = useState(false);
  const [ghSetupModal, setGhSetupModal] = useState<{ reason: GitHubCliSetupReason } | null>(null);

  const gitAvailable = Boolean(changes?.gitAvailable);

  useEffect(() => {
    setQuery("");
    setLastStatus(null);
    setGhProbe(null);
  }, [cwd]);

  const refreshGhProbe = useCallback(async () => {
    if (!gitAvailable) {
      setGhProbe(null);
      return;
    }
    setGhProbing(true);
    try {
      const probe = await probeGitHubCli((command) => app.RunShellQuiet(command));
      setGhProbe(probe);
    } finally {
      setGhProbing(false);
    }
  }, [gitAvailable]);

  useEffect(() => {
    void refreshGhProbe();
  }, [refreshGhProbe, refreshKey]);

  useEffect(() => {
    const onGitPrefs = () => void refreshGhProbe();
    window.addEventListener("arcdesk:desktop-git-settings", onGitPrefs);
    return () => window.removeEventListener("arcdesk:desktop-git-settings", onGitPrefs);
  }, [refreshGhProbe]);

  useDismissOnClickOutside(Boolean(rowMenu), () => setRowMenu(null));

  const gitRows = useMemo(() => sortGitRows((changes?.files ?? []).filter(hasGit)), [changes?.files]);

  const filteredRows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return gitRows;
    return gitRows.filter((row) =>
      `${row.path} ${row.oldPath ?? ""} ${row.gitStatus ?? ""}`.toLowerCase().includes(q),
    );
  }, [gitRows, query]);

  const runGit = useCallback(
    async (command: string) => {
      setGitBusy(true);
      setLastStatus(null);
      try {
        const result = await app.RunShellQuiet(command);
        setLastStatus({ command, output: result.output, err: result.err });
        await loadChanges();
        return result;
      } finally {
        setGitBusy(false);
      }
    },
    [loadChanges],
  );

  const stageAll = () => void runGit("git add -A");
  const pull = () => void runGit("git pull");
  const push = () => void runGit("git push");

  const stageFile = (path: string) => {
    setRowMenu(null);
    void runGit(`git add -- ${shellQuote(path)}`);
  };

  const unstageFile = (path: string) => {
    setRowMenu(null);
    void runGit(`git restore --staged -- ${shellQuote(path)}`);
  };

  const discardFile = (path: string) => {
    setRowMenu(null);
    void runGit(`git restore -- ${shellQuote(path)}`);
  };

  const commitChanges = () => {
    const message = commitMessage.trim();
    if (!message) return;
    void runGit(`git commit -m ${shellQuote(message)}`).then((result) => {
      if (!result?.err) setCommitMessage("");
    });
  };

  const suggestCommitMessage = () => {
    if (gitRows.length === 0) return;
    onAddToChat?.(buildCommitMessagePrompt(formatGitRowsForPrompt(gitRows), t));
  };

  const suggestPullRequest = () => {
    if (gitRows.length === 0) return;
    onAddToChat?.(buildPRPrompt(formatGitRowsForPrompt(gitRows), t));
  };

  const mergePullRequest = () => {
    if (ghProbe?.canMerge) {
      void runGit(ghPRMergeCommand()).then((result) => {
        if (!result?.err) void refreshGhProbe();
      });
      return;
    }
    const reason: GitHubCliSetupReason =
      !isGitHubCliCheckEnabled() ? "setup_required" : (ghProbe?.reason ?? "no_pr");
    setGhSetupModal({ reason });
  };

  const mergeTooltip = useMemo(() => {
    if (ghProbing) return t("git.ghProbing");
    if (ghProbe?.canMerge && ghProbe.prNumber != null) {
      return t("git.ghPrReady", {
        number: String(ghProbe.prNumber),
        title: ghProbe.prTitle ?? "",
      });
    }
    const key = probeReasonKey(ghProbe?.reason ?? null);
    if (key) return t(key);
    return t("git.mergePR");
  }, [ghProbe, ghProbing, t]);

  const ghBannerKey = useMemo(() => probeReasonKey(ghProbe?.reason ?? null), [ghProbe?.reason]);

  const openRowMenu = (event: ReactMouseEvent<HTMLElement>, row: WorkspaceChangeView) => {
    event.preventDefault();
    event.stopPropagation();
    setRowMenu({ x: event.clientX, y: event.clientY, row });
  };

  const addReferenceToChat = (path: string) => {
    onAddToChat?.(formatWorkspaceReference(path, false));
    setRowMenu(null);
  };

  const addFileContentToChat = async (path: string) => {
    setRowMenu(null);
    if (!onAddToChat) return;
    await addWorkspaceFileContentToChat(path, onAddToChat, t("workspace.truncated"));
  };

  const statusSummary = lastStatus
    ? lastStatus.err
      ? lastStatus.err
      : summarizeOutput(lastStatus.output) || t("git.statusDone")
    : "";

  return (
    <div className="dock-panel git-panel">
      <header className="dock-panel__head">
        <div className="dock-panel__head-main">
          <h2 className="dock-panel__title">{t("rightDock.tab.git")}</h2>
          <Tooltip label={cwd ?? undefined}>
            <p className="dock-panel__meta">{shortCwd(cwd) || t("workspace.title")}</p>
          </Tooltip>
        </div>
        <Tooltip label={t("git.refresh")}>
          <button type="button" className="dock-panel__ghost" onClick={() => void loadChanges()} aria-label={t("git.refresh")}>
            <RefreshCw size={14} strokeWidth={1.75} className={loading ? "dock-panel__spin" : undefined} />
          </button>
        </Tooltip>
      </header>

      {changes && !changes.gitAvailable && changes.gitErr && (
        <p className="dock-panel__banner dock-panel__banner--warn">{t("workspace.gitUnavailable")}</p>
      )}

      {gitAvailable && ghBannerKey && (
        <p className="dock-panel__banner dock-panel__banner--warn">{t(ghBannerKey)}</p>
      )}

      {gitAvailable && (
        <section className="git-panel__workflow" aria-label={t("rightDock.tab.git")}>
          <div className="git-panel__remote">
            <button type="button" className="git-panel__remote-btn" onClick={pull} disabled={gitBusy}>
              <ArrowDownToLine size={14} strokeWidth={1.75} />
              {t("git.pull")}
            </button>
            <span className="git-panel__remote-sep" aria-hidden="true" />
            <button type="button" className="git-panel__remote-btn" onClick={push} disabled={gitBusy}>
              <ArrowUpFromLine size={14} strokeWidth={1.75} />
              {t("git.push")}
            </button>
            <span className="git-panel__remote-sep" aria-hidden="true" />
            <Tooltip label={mergeTooltip}>
              <button
                type="button"
                className="git-panel__remote-btn"
                onClick={mergePullRequest}
                disabled={gitBusy || ghProbing}
              >
                <GitCommitHorizontal size={14} strokeWidth={1.75} />
                {t("git.mergePR")}
              </button>
            </Tooltip>
          </div>

          <div className="git-panel__commit">
            <label className="git-panel__commit-label" htmlFor="git-commit-message">
              {t("git.commitLabel")}
            </label>
            <textarea
              id="git-commit-message"
              className="git-panel__message"
              value={commitMessage}
              onChange={(event) => setCommitMessage(event.target.value)}
              placeholder={t("git.commitPlaceholder")}
              disabled={gitBusy}
              rows={2}
              onKeyDown={(event) => {
                if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                  event.preventDefault();
                  commitChanges();
                }
              }}
            />
            <div className="git-panel__commit-foot">
              <div className="git-panel__commit-links">
                <button type="button" className="git-panel__text-btn" onClick={stageAll} disabled={gitBusy}>
                  {t("git.stageAll")}
                </button>
                <button
                  type="button"
                  className="git-panel__text-btn"
                  onClick={suggestCommitMessage}
                  disabled={gitBusy || gitRows.length === 0}
                >
                  <Sparkles size={12} strokeWidth={1.75} />
                  {t("git.suggestCommit")}
                </button>
                <button
                  type="button"
                  className="git-panel__text-btn"
                  onClick={suggestPullRequest}
                  disabled={gitBusy || gitRows.length === 0}
                >
                  <Sparkles size={12} strokeWidth={1.75} />
                  {t("git.suggestPR")}
                </button>
              </div>
              <button
                type="button"
                className="git-panel__commit-btn"
                onClick={commitChanges}
                disabled={gitBusy || !commitMessage.trim()}
              >
                <GitCommitHorizontal size={14} strokeWidth={1.75} />
                {t("git.commit")}
              </button>
            </div>
            <p className="git-panel__hint">
              {t("git.commitHint")} <kbd className="git-panel__kbd">⌘</kbd> <kbd className="git-panel__kbd">↵</kbd>
            </p>
          </div>

          {(gitBusy || lastStatus) && (
            <p
              className={`git-panel__feed${lastStatus?.err ? " git-panel__feed--err" : ""}${gitBusy ? " git-panel__feed--busy" : ""}`}
              role="status"
            >
              {gitBusy ? (
                <>
                  <Loader2 size={12} className="dock-panel__spin" />
                  {t("git.statusRunning")}
                </>
              ) : (
                <>
                  {lastStatus?.err ? <X size={12} /> : <Check size={12} />}
                  <code>{lastStatus?.command}</code>
                  <span>{statusSummary}</span>
                </>
              )}
            </p>
          )}
        </section>
      )}

      <section className="git-panel__changes" aria-label={t("git.changesHeading")}>
        <div className="dock-panel__section-head">
          <div className="dock-panel__section-title">
            <span>{t("git.changesHeading")}</span>
            <span className="dock-panel__count">{gitRows.length}</span>
          </div>
          <input
            className="dock-panel__filter"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("git.filter")}
            aria-label={t("git.filter")}
          />
        </div>

        <ul className="dock-panel__list">
          {loading && !changes ? (
            <li className="dock-panel__empty">{t("workspace.loadingChanges")}</li>
          ) : filteredRows.length === 0 ? (
            <li className={`dock-panel__empty${gitRows.length > 0 ? " dock-panel__search-empty" : ""}`}>
              <span>{gitRows.length > 0 ? t("workspace.noSearchResults") : t("git.empty")}</span>
              <small>{gitRows.length > 0 ? t("workspace.noSearchResultsHint") : t("git.emptyHint")}</small>
            </li>
          ) : (
            filteredRows.map((row) => {
              const deleted = isDeletedChange(row);
              const active = activeFilePath === row.path;
              const tone = row.gitStatus ? gitStatusTone(row.gitStatus) : "mod";
              return (
                <li key={row.path}>
                  <button
                    type="button"
                    className={`dock-panel__row${active ? " dock-panel__row--active" : ""}${deleted ? " dock-panel__row--deleted" : ""}`}
                    onContextMenu={(event) => openRowMenu(event, row)}
                    onClick={() => {
                      if (!deleted) onOpenFile?.(row.path);
                    }}
                  >
                    {row.gitStatus && (
                      <span className={`dock-panel__pill dock-panel__pill--git-${tone}`}>{row.gitStatus.trim()}</span>
                    )}
                    <span className="dock-panel__row-copy">
                      <span className="dock-panel__row-name">{basename(row.path)}</span>
                      <span className="dock-panel__row-path">{row.path}</span>
                    </span>
                  </button>
                </li>
              );
            })
          )}
        </ul>
      </section>

      {rowMenu && gitAvailable && (
        <FloatingMenu x={rowMenu.x} y={rowMenu.y} estimatedHeight={ROW_MENU_HEIGHT} className="workspace-tree-menu" onClose={() => setRowMenu(null)}>
          <FloatingMenuItems
            items={[
              {
                icon: <Plus size={14} />,
                label: t("git.stageFile"),
                onSelect: () => stageFile(rowMenu.row.path),
              },
              {
                icon: <Undo2 size={14} />,
                label: t("git.unstageFile"),
                onSelect: () => unstageFile(rowMenu.row.path),
              },
              {
                icon: <RotateCcw size={14} />,
                label: t("git.discardFile"),
                onSelect: () => discardFile(rowMenu.row.path),
              },
              {
                icon: <MessageSquarePlus size={14} />,
                label: t("workspace.addFileReferenceToChat"),
                onSelect: () => addReferenceToChat(rowMenu.row.path),
              },
              {
                icon: <MessageSquarePlus size={14} />,
                label: t("workspace.addFileContentToChat"),
                onSelect: () => void addFileContentToChat(rowMenu.row.path),
              },
            ]}
          />
        </FloatingMenu>
      )}

      {ghSetupModal && (
        <GitHubCliSetupModal
          reason={ghSetupModal.reason}
          checkEnabled={isGitHubCliCheckEnabled()}
          onClose={() => setGhSetupModal(null)}
          onOpenSettings={() => openGitHubCliSettings({ runCheck: true, enableCheck: true })}
        />
      )}
    </div>
  );
}
