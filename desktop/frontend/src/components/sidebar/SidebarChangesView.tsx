import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, Loader2, RotateCcw } from "lucide-react";
import { app } from "../../lib/bridge";
import { useT } from "../../lib/i18n";
import { shellQuote } from "../../lib/shellQuote";
import { useGitFileDiff } from "../../lib/useGitFileDiff";
import { useWorkspaceChanges } from "../../lib/useWorkspaceChanges";
import type { WorkspaceChangeView } from "../../lib/types";
import { hasGitChange } from "../../lib/workspaceChangeHelpers";
import { DockEmptyState } from "../dock/DockEmptyState";
import { GitRepoInitBanner } from "../dock/GitRepoInitBanner";

interface SidebarChangesViewProps {
  cwd?: string;
  refreshKey?: number;
  activeFilePath?: string | null;
  excluded: Set<string>;
  onExcludedChange: (next: Set<string>) => void;
  onOpenFile?: (path: string) => void;
  onStatsChange?: (stats: { count: number; added: number; removed: number }) => void;
  onRegisterStage?: (stage: () => Promise<void>) => void;
}

function sortGitRows(rows: WorkspaceChangeView[]): WorkspaceChangeView[] {
  return [...rows].sort((a, b) => a.path.localeCompare(b.path));
}

function ChangeDiffCard({
  row,
  active,
  confirmed,
  onToggleConfirm,
  onDiscard,
  onOpen,
}: {
  row: WorkspaceChangeView;
  active: boolean;
  confirmed: boolean;
  onToggleConfirm: () => void;
  onDiscard: () => void;
  onOpen: () => void;
}) {
  const t = useT();
  const diff = useGitFileDiff(row.path, row.gitStatus, true);
  const isNew = row.gitStatus?.includes("?") || row.gitStatus?.includes("A");

  return (
    <article className={`sidebar-change-card${active ? " sidebar-change-card--active" : ""}`}>
      <header className="sidebar-change-card__head">
        <button type="button" className="sidebar-change-card__open" onClick={onOpen}>
          <span className="sidebar-change-card__path">{row.path}</span>
          <span className="sidebar-change-card__meta">
            {isNew ? <span className="sidebar-change-card__tag sidebar-change-card__tag--new">{t("sidebar.fileNew")}</span> : null}
            {row.gitStatus ? <span className="sidebar-change-card__tag">{row.gitStatus.trim()}</span> : null}
            {diff.loading ? (
              <Loader2 size={12} className="spin" aria-hidden="true" />
            ) : (
              <>
                {diff.added > 0 ? <span className="sidebar-change-card__stat sidebar-change-card__stat--add">+{diff.added}</span> : null}
                {diff.removed > 0 ? <span className="sidebar-change-card__stat sidebar-change-card__stat--del">-{diff.removed}</span> : null}
              </>
            )}
          </span>
        </button>
        <div className="sidebar-change-card__actions">
          <button
            type="button"
            className="sidebar-change-card__action"
            onClick={onDiscard}
            aria-label={t("git.discardFile")}
            title={t("git.discardFile")}
          >
            <RotateCcw size={13} />
          </button>
          <label className="sidebar-change-card__confirm" title={t("sidebar.confirmChange")}>
            <input type="checkbox" checked={confirmed} onChange={onToggleConfirm} />
            <Check size={12} aria-hidden="true" />
          </label>
        </div>
      </header>
      {diff.lines.length > 0 ? (
        <pre className="sidebar-change-card__diff">
          {diff.lines.map((line) => (
            <div key={`${line.no}-${line.kind}`} className={`sidebar-change-card__line sidebar-change-card__line--${line.kind}`}>
              <span className="sidebar-change-card__ln">{line.no}</span>
              <code>{line.text || " "}</code>
            </div>
          ))}
        </pre>
      ) : diff.loading ? (
        <p className="sidebar-change-card__loading">{t("sidebar.loadingDiff")}</p>
      ) : null}
    </article>
  );
}

export function SidebarChangesView({
  cwd,
  refreshKey,
  activeFilePath,
  excluded,
  onExcludedChange,
  onOpenFile,
  onStatsChange,
  onRegisterStage,
}: SidebarChangesViewProps) {
  const t = useT();
  const { changes, loading, loadChanges } = useWorkspaceChanges(cwd, refreshKey);
  const gitRows = useMemo(() => sortGitRows((changes?.files ?? []).filter(hasGitChange)), [changes?.files]);
  const [busyPath, setBusyPath] = useState<string | null>(null);

  useEffect(() => {
    onExcludedChange(new Set());
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reset when workspace refreshes
  }, [cwd, refreshKey]);

  const stats = useMemo(() => ({ count: gitRows.length, added: 0, removed: 0 }), [gitRows.length]);

  useEffect(() => {
    onStatsChange?.(stats);
  }, [onStatsChange, stats]);

  const isConfirmed = useCallback((path: string) => !excluded.has(path), [excluded]);

  const toggleConfirm = (path: string) => {
    const next = new Set(excluded);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    onExcludedChange(next);
  };

  const discardFile = async (path: string) => {
    setBusyPath(path);
    try {
      await app.RunShellQuiet(`git restore -- ${shellQuote(path)}`);
      const next = new Set(excluded);
      next.delete(path);
      onExcludedChange(next);
      await loadChanges();
    } finally {
      setBusyPath(null);
    }
  };

  const stageConfirmed = useCallback(async () => {
    const toStage = gitRows.filter((row) => isConfirmed(row.path)).map((row) => row.path);
    if (toStage.length === 0) return;
    for (const path of toStage) {
      await app.RunShellQuiet(`git add -- ${shellQuote(path)}`);
    }
    await loadChanges();
  }, [gitRows, isConfirmed, loadChanges]);

  useEffect(() => {
    onRegisterStage?.(stageConfirmed);
  }, [onRegisterStage, stageConfirmed]);

  return (
    <div className="sidebar-changes-view">
      <GitRepoInitBanner
        cwd={cwd}
        gitAvailable={changes?.gitAvailable}
        gitErr={changes?.gitErr}
        onInitialized={() => void loadChanges()}
      />

      <div className="sidebar-changes-view__list">
        {loading && !changes ? (
          <p className="dock-panel__empty">{t("workspace.loadingChanges")}</p>
        ) : gitRows.length === 0 ? (
          <DockEmptyState title={t("git.empty")} hint={t("git.emptyHint")} />
        ) : (
          gitRows.map((row) => (
            <ChangeDiffCard
              key={row.path}
              row={row}
              active={activeFilePath === row.path}
              confirmed={isConfirmed(row.path)}
              onToggleConfirm={() => toggleConfirm(row.path)}
              onDiscard={() => void discardFile(row.path)}
              onOpen={() => onOpenFile?.(row.path)}
            />
          ))
        )}
      </div>

      {busyPath ? <p className="sidebar-changes-view__busy">{t("git.statusRunning")}</p> : null}
    </div>
  );
}
