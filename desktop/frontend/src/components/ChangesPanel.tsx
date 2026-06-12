import { useEffect, useMemo, useState } from "react";
import type { DragEvent as ReactDragEvent, MouseEvent as ReactMouseEvent } from "react";
import { MessageSquarePlus, Search } from "lucide-react";
import { useT } from "../lib/i18n";
import { useDismissOverlay } from "../lib/useDismissOverlay";
import { useWorkspaceChanges } from "../lib/useWorkspaceChanges";
import type { WorkspaceChangeView } from "../lib/types";
import { basename, shortCwd } from "../lib/workspaceFilePreview";
import {
  addWorkspaceFileToChat,
  addWorkspaceReferenceToChat,
  openWorkspaceRowMenu,
  startWorkspaceDrag,
} from "../lib/workspaceDockActions";
import { DockEmptyState } from "./dock/DockEmptyState";
import { DockPanelHeader } from "./dock/DockPanelHeader";
import { FloatingMenu, FloatingMenuItems } from "./FloatingMenu";
import { CodeReviewSection, type CodeReviewState } from "./CodeReviewSection";
import type { ReviewMode, ReviewScope } from "../lib/codeReview";
import {
  getCodeReviewDefaultScope,
  getCodeReviewSecurityByDefault,
} from "../lib/codeReviewPrefs";
import { CODE_REVIEW_SETTINGS_EVENT } from "../lib/events";
import { hasGitChange, isDeletedGitChange } from "../lib/workspaceChangeHelpers";

type SourceFilter = ReviewScope;

interface ChangesPanelProps {
  cwd?: string;
  refreshKey?: number;
  activeFilePath?: string | null;
  running?: boolean;
  review?: CodeReviewState;
  onOpenFile?: (path: string) => void;
  onAddToChat?: (text: string) => void;
  onRunReview?: (mode: ReviewMode, scope: ReviewScope, paths: string[]) => void;
  onClearReview?: () => void;
}

function hasSession(row: WorkspaceChangeView): boolean {
  return row.sources.includes("session");
}

function matchesSourceFilter(row: WorkspaceChangeView, filter: SourceFilter): boolean {
  if (filter === "all") return true;
  if (filter === "session") return hasSession(row);
  if (filter === "git") return hasGitChange(row);
  return hasSession(row) && hasGitChange(row);
}

function sortChanges(rows: WorkspaceChangeView[]): WorkspaceChangeView[] {
  return [...rows].sort((a, b) => {
    const aBoth = hasSession(a) && hasGitChange(a) ? 1 : 0;
    const bBoth = hasSession(b) && hasGitChange(b) ? 1 : 0;
    if (aBoth !== bBoth) return bBoth - aBoth;
    const aTime = a.latestTime ?? 0;
    const bTime = b.latestTime ?? 0;
    if (aTime !== bTime) return bTime - aTime;
    return a.path.localeCompare(b.path);
  });
}

function changeDetail(row: WorkspaceChangeView): string {
  if (row.latestPrompt) return row.latestPrompt;
  if (row.oldPath) return `← ${row.oldPath}`;
  if (row.turns && row.turns.length > 0) return `#${row.turns.join(", #")}`;
  return "";
}

const WORKSPACE_CONTEXT_MENU_FILE_HEIGHT = 92;

export function ChangesPanel({
  cwd,
  refreshKey,
  activeFilePath,
  running = false,
  review,
  onOpenFile,
  onAddToChat,
  onRunReview,
  onClearReview,
}: ChangesPanelProps) {
  const t = useT();
  const { changes, loading: loadingChanges, loadChanges } = useWorkspaceChanges(cwd, refreshKey);
  const [query, setQuery] = useState("");
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>(() => getCodeReviewDefaultScope());
  const [reviewMode, setReviewMode] = useState<ReviewMode>(() =>
    getCodeReviewSecurityByDefault() ? "security" : "standard",
  );
  const [rowMenu, setRowMenu] = useState<{ x: number; y: number; path: string } | null>(null);

  useEffect(() => {
    const syncFromSettings = () => {
      setSourceFilter(getCodeReviewDefaultScope());
      setReviewMode(getCodeReviewSecurityByDefault() ? "security" : "standard");
    };
    window.addEventListener(CODE_REVIEW_SETTINGS_EVENT, syncFromSettings);
    return () => window.removeEventListener(CODE_REVIEW_SETTINGS_EVENT, syncFromSettings);
  }, []);

  useEffect(() => {
    setQuery("");
    setSourceFilter(getCodeReviewDefaultScope());
  }, [cwd]);

  useDismissOverlay(Boolean(rowMenu), () => setRowMenu(null), { mode: "click" });

  const allRows = useMemo(() => sortChanges(changes?.files ?? []), [changes?.files]);

  const scopeRows = useMemo(
    () => allRows.filter((row) => matchesSourceFilter(row, sourceFilter)),
    [allRows, sourceFilter],
  );

  const filteredRows = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return scopeRows;
    return scopeRows.filter((row) =>
      `${row.path} ${row.oldPath ?? ""} ${row.gitStatus ?? ""} ${row.latestPrompt ?? ""} ${(row.turns ?? []).join(" ")}`
        .toLowerCase()
        .includes(q),
    );
  }, [scopeRows, query]);

  const openFile = (path: string) => {
    onOpenFile?.(path);
    setRowMenu(null);
  };

  const startRowDrag = (event: ReactDragEvent<HTMLElement>, path: string) => {
    startWorkspaceDrag(event, path);
  };

  const openRowMenu = (event: ReactMouseEvent<HTMLElement>, path: string) => {
    openWorkspaceRowMenu<{ x: number; y: number; path: string }>(event, { path }, (menu) => setRowMenu(menu));
  };

  const addReferenceToChat = (path: string) => {
    void addWorkspaceReferenceToChat(path, onAddToChat, () => setRowMenu(null));
  };

  const addFileContentToChat = async (path: string) => {
    await addWorkspaceFileToChat(path, onAddToChat, () => setRowMenu(null), t("workspace.truncated"));
  };

  return (
    <div className="dock-panel changes-panel">
      <DockPanelHeader
        title={t("workspace.changedTab")}
        cwd={cwd}
        cwdLabel={shortCwd(cwd) || t("workspace.title")}
        refreshLabel={t("workspace.refreshChanges")}
        refreshing={loadingChanges}
        onRefresh={() => void loadChanges()}
      />

      {changes && !changes.gitAvailable && changes.gitErr && (
        <p className="dock-panel__banner dock-panel__banner--warn">{t("workspace.gitUnavailable")}</p>
      )}

      <CodeReviewSection
        scope={sourceFilter}
        fileCount={scopeRows.length}
        running={running}
        mode={reviewMode}
        review={{
          status: review?.status ?? "idle",
          mode: reviewMode,
          scope: sourceFilter,
          text: review?.text,
          error: review?.error,
          finishedAt: review?.finishedAt,
        }}
        onScopeChange={setSourceFilter}
        onModeChange={setReviewMode}
        onRun={() => onRunReview?.(reviewMode, sourceFilter, scopeRows.map((row) => row.path))}
        onClear={() => onClearReview?.()}
        onOpenFile={(path) => openFile(path)}
      />

      <div className="dock-panel__section-head">
        <div className="dock-panel__section-title">
          <span>{t("changes.listHeading")}</span>
          <span className="dock-panel__count">{scopeRows.length}</span>
        </div>
        <label className="dock-panel__filter-wrap">
          <Search size={13} strokeWidth={1.75} className="dock-panel__filter-ico" aria-hidden="true" />
          <input
            className="dock-panel__filter"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("workspace.filterChanges")}
            aria-label={t("workspace.filterChanges")}
          />
        </label>
      </div>

      <ul className="dock-panel__list">
        {loadingChanges && !changes ? (
          <li className="dock-panel__empty">{t("workspace.loadingChanges")}</li>
        ) : filteredRows.length === 0 ? (
          <DockEmptyState
            title={scopeRows.length > 0 ? t("workspace.noSearchResults") : t("workspace.noChanges")}
            hint={scopeRows.length > 0 ? t("workspace.noSearchResultsHint") : t("changes.emptyHint")}
            searchMode={scopeRows.length > 0}
          />
        ) : (
          filteredRows.map((row) => {
            const deleted = isDeletedGitChange(row);
            const active = activeFilePath === row.path;
            const detail = changeDetail(row);
            return (
              <li key={`${row.path}-${row.sources.join("-")}`}>
                <button
                  type="button"
                  className={`dock-panel__row${active ? " dock-panel__row--active" : ""}${deleted ? " dock-panel__row--deleted" : ""}`}
                  draggable={!deleted}
                  onDragStart={(event) => startRowDrag(event, row.path)}
                  onContextMenu={(event) => openRowMenu(event, row.path)}
                  onClick={() => {
                    if (!deleted) openFile(row.path);
                  }}
                >
                  <span className="dock-panel__row-copy">
                    <span className="dock-panel__row-name">{basename(row.path)}</span>
                    <span className="dock-panel__row-path">{row.path}</span>
                    {detail && <span className="dock-panel__row-detail">{detail}</span>}
                  </span>
                  <span className="dock-panel__row-tags">
                    {row.gitStatus && <span className="dock-panel__pill dock-panel__pill--git">{row.gitStatus.trim()}</span>}
                    {hasSession(row) && <span className="dock-panel__pill dock-panel__pill--session">{t("workspace.sourceSession")}</span>}
                    {hasGitChange(row) && !row.gitStatus && <span className="dock-panel__pill dock-panel__pill--git">{t("workspace.sourceGit")}</span>}
                    {deleted && <span className="dock-panel__pill dock-panel__pill--del">{t("workspace.deleted")}</span>}
                  </span>
                </button>
              </li>
            );
          })
        )}
      </ul>

      {rowMenu && (
        <FloatingMenu
          x={rowMenu.x}
          y={rowMenu.y}
          estimatedHeight={WORKSPACE_CONTEXT_MENU_FILE_HEIGHT}
          className="workspace-tree-menu"
          onClose={() => setRowMenu(null)}
        >
          <FloatingMenuItems
            items={[
              {
                icon: <MessageSquarePlus size={14} />,
                label: t("workspace.addFileReferenceToChat"),
                onSelect: () => addReferenceToChat(rowMenu.path),
              },
              {
                icon: <MessageSquarePlus size={14} />,
                label: t("workspace.addFileContentToChat"),
                onSelect: () => void addFileContentToChat(rowMenu.path),
              },
            ]}
          />
        </FloatingMenu>
      )}
    </div>
  );
}
