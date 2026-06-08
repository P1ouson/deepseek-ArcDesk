import { useEffect, useMemo, useState } from "react";
import type { DragEvent as ReactDragEvent, MouseEvent as ReactMouseEvent } from "react";
import { MessageSquarePlus, RefreshCw, Search } from "lucide-react";
import { useT } from "../lib/i18n";
import { useDismissOnClickOutside } from "../lib/useDismissOnClickOutside";
import { useWorkspaceChanges } from "../lib/useWorkspaceChanges";
import type { WorkspaceChangeView } from "../lib/types";
import { addWorkspaceFileContentToChat } from "../lib/workspaceAddToChat";
import { basename, shortCwd } from "../lib/workspaceFilePreview";
import { formatWorkspaceReference, WORKSPACE_REF_DRAG_TYPE } from "../lib/workspaceDrag";
import { FloatingMenu, FloatingMenuItems } from "./FloatingMenu";
import { Tooltip } from "./Tooltip";
import { CodeReviewSection, type CodeReviewState } from "./CodeReviewSection";
import type { ReviewMode, ReviewScope } from "../lib/codeReview";
import {
  getCodeReviewDefaultScope,
  getCodeReviewSecurityByDefault,
} from "../lib/codeReviewPrefs";

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

function isDeletedChange(row: WorkspaceChangeView): boolean {
  return !!row.gitStatus && row.gitStatus.includes("D");
}

function hasSession(row: WorkspaceChangeView): boolean {
  return row.sources.includes("session");
}

function hasGit(row: WorkspaceChangeView): boolean {
  return row.sources.includes("git");
}

function matchesSourceFilter(row: WorkspaceChangeView, filter: SourceFilter): boolean {
  if (filter === "all") return true;
  if (filter === "session") return hasSession(row);
  if (filter === "git") return hasGit(row);
  return hasSession(row) && hasGit(row);
}

function sortChanges(rows: WorkspaceChangeView[]): WorkspaceChangeView[] {
  return [...rows].sort((a, b) => {
    const aBoth = hasSession(a) && hasGit(a) ? 1 : 0;
    const bBoth = hasSession(b) && hasGit(b) ? 1 : 0;
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
    window.addEventListener("arcdesk:code-review-settings", syncFromSettings);
    return () => window.removeEventListener("arcdesk:code-review-settings", syncFromSettings);
  }, []);

  useEffect(() => {
    setQuery("");
    setSourceFilter(getCodeReviewDefaultScope());
  }, [cwd]);

  useDismissOnClickOutside(Boolean(rowMenu), () => setRowMenu(null));

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
    const ref = formatWorkspaceReference(path, false);
    event.dataTransfer.effectAllowed = "copy";
    event.dataTransfer.setData(WORKSPACE_REF_DRAG_TYPE, JSON.stringify({ path, isDir: false }));
    event.dataTransfer.setData("text/plain", ref);
  };

  const openRowMenu = (event: ReactMouseEvent<HTMLElement>, path: string) => {
    event.preventDefault();
    event.stopPropagation();
    setRowMenu({ x: event.clientX, y: event.clientY, path });
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

  return (
    <div className="dock-panel changes-panel">
      <header className="dock-panel__head">
        <div className="dock-panel__head-main">
          <h2 className="dock-panel__title">{t("workspace.changedTab")}</h2>
          <Tooltip label={cwd ?? undefined}>
            <p className="dock-panel__meta">{shortCwd(cwd) || t("workspace.title")}</p>
          </Tooltip>
        </div>
        <Tooltip label={t("workspace.refreshChanges")}>
          <button type="button" className="dock-panel__ghost" onClick={() => void loadChanges()} aria-label={t("workspace.refreshChanges")}>
            <RefreshCw size={14} strokeWidth={1.75} className={loadingChanges ? "dock-panel__spin" : undefined} />
          </button>
        </Tooltip>
      </header>

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
          <li className={`dock-panel__empty${scopeRows.length > 0 ? " dock-panel__search-empty" : ""}`}>
            <span>{scopeRows.length > 0 ? t("workspace.noSearchResults") : t("workspace.noChanges")}</span>
            <small>{scopeRows.length > 0 ? t("workspace.noSearchResultsHint") : t("changes.emptyHint")}</small>
          </li>
        ) : (
          filteredRows.map((row) => {
            const deleted = isDeletedChange(row);
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
                    {hasGit(row) && !row.gitStatus && <span className="dock-panel__pill dock-panel__pill--git">{t("workspace.sourceGit")}</span>}
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
