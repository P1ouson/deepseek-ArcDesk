import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  Check,
  ChevronDown,
  ChevronLeft,
  FileText,
  Folder,
  FolderX,
  FolderOpen,
  Pencil,
  Plus,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { app } from "../lib/bridge";
import { confirmAction } from "../lib/confirmAction";
import { getLocale, useT } from "../lib/i18n";
import type { FileEntry } from "../lib/types";
import {
  isPathUnderRoot,
  normalizeWritePath,
  parentWritePath,
  pathsEqual,
} from "../lib/writePaths";
import { getRecentWorkspacePaths, recordRecentWorkspace } from "../lib/workspaceRecents";
import { isNoWriteWorkspace, NO_WORKSPACE_VALUE } from "../lib/writeWorkspace";
import { closeStudioSelect, openStudioSelect } from "../lib/studioSelectRegistry";
import { AnchoredPopover } from "./AnchoredPopover";
import { FileTypeIcon } from "./FileTypeIcon";
import { Tooltip } from "./Tooltip";

const PICK_WORKSPACE_VALUE = "__pick_workspace__";
const PICK_FILE_VALUE = "__pick_file__";

function WriteFileListPane({ browsePath, children }: { browsePath: string; children: ReactNode }) {
  return (
    <div key={browsePath} className="write-studio__file-list-scroll">
      {children}
    </div>
  );
}

export interface WriteSidebarProps {
  workspaceRoot: string;
  selectedPath?: string;
  dirty?: boolean;
  onSelectFile: (path: string) => void;
  onWorkspaceChange?: (root: string) => void;
  onPickWorkspace?: () => Promise<string | undefined>;
  onPickFile?: () => Promise<string | undefined>;
  onFilesChanged?: () => void;
}

function baseName(path: string): string {
  return path.replace(/[/\\]+$/, "").split(/[/\\]/).filter(Boolean).pop() ?? path;
}

function isWriteListHiddenFile(name: string): boolean {
  const base = name.trim();
  if (!base) return true;
  if (base.startsWith("~$")) return true;
  if (base.startsWith("~WRL") || base.startsWith("~WRD")) return true;
  if (base.startsWith(".")) return true;
  return false;
}

function formatModTime(ms?: number): string {
  if (!ms) return "";
  const locale = getLocale() === "zh" ? "zh-CN" : "en";
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  const delta = Date.now() - ms;
  const minute = 60_000;
  const hour = 60 * minute;
  const day = 24 * hour;
  if (delta < minute) return rtf.format(0, "minute");
  if (delta < hour) return rtf.format(-Math.max(1, Math.round(delta / minute)), "minute");
  if (delta < day) return rtf.format(-Math.round(delta / hour), "hour");
  if (delta < 7 * day) return rtf.format(-Math.round(delta / day), "day");
  return new Date(ms).toLocaleDateString(locale);
}

export function WriteSidebar({
  workspaceRoot,
  selectedPath,
  dirty = false,
  onSelectFile,
  onWorkspaceChange,
  onPickWorkspace,
  onPickFile,
  onFilesChanged,
}: WriteSidebarProps) {
  const t = useT();
  const workspaceAnchorRef = useRef<HTMLButtonElement>(null);
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const closeWorkspaceMenu = useCallback(() => setWorkspaceMenuOpen(false), []);
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [browsePath, setBrowsePathState] = useState(() => normalizeWritePath(workspaceRoot));
  const [busy, setBusy] = useState(false);
  const noWorkspace = isNoWriteWorkspace(workspaceRoot);

  const setBrowsePath = useCallback((path: string) => {
    setBrowsePathState(normalizeWritePath(path));
  }, []);

  const recentWorkspaces = useMemo(() => {
    const current = normalizeWritePath(workspaceRoot);
    return getRecentWorkspacePaths()
      .filter((path) => normalizeWritePath(path) !== current)
      .slice(0, 5);
  }, [workspaceRoot]);

  useEffect(() => {
    setBrowsePath(workspaceRoot);
  }, [setBrowsePath, workspaceRoot]);

  useEffect(() => {
    if (!selectedPath || noWorkspace) return;
    const file = normalizeWritePath(selectedPath);
    const root = normalizeWritePath(workspaceRoot);
    if (!isPathUnderRoot(file, root)) return;
    setBrowsePath(parentWritePath(file));
  }, [noWorkspace, selectedPath, setBrowsePath, workspaceRoot]);

  useEffect(() => {
    if (!workspaceMenuOpen) return;
    openStudioSelect(closeWorkspaceMenu);
    return () => closeStudioSelect(closeWorkspaceMenu);
  }, [workspaceMenuOpen, closeWorkspaceMenu]);

  const reload = useCallback(async () => {
    if (noWorkspace) {
      setEntries([]);
      return;
    }
    setBusy(true);
    try {
      const listed = await app.ListWriteDir(workspaceRoot, browsePath).catch(() => [] as FileEntry[]);
      setEntries(listed);
    } finally {
      setBusy(false);
    }
  }, [browsePath, noWorkspace, workspaceRoot]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const visibleEntries = useMemo(
    () => entries.filter((entry) => !isWriteListHiddenFile(entry.name)),
    [entries],
  );

  const breadcrumbParts = useMemo(() => {
    const root = normalizeWritePath(workspaceRoot);
    const current = normalizeWritePath(browsePath);
    if (pathsEqual(current, root)) return [{ path: root, label: baseName(root) || t("write.sidebar.currentFolder") }];
    if (!isPathUnderRoot(current, root)) {
      return [{ path: root, label: baseName(root) || t("write.sidebar.currentFolder") }];
    }
    const rel = current.slice(root.length + 1);
    const parts = [{ path: root, label: baseName(root) || t("write.sidebar.currentFolder") }];
    let acc = root;
    for (const segment of rel.split("/").filter(Boolean)) {
      acc = `${acc}/${segment}`;
      parts.push({ path: acc, label: segment });
    }
    return parts;
  }, [browsePath, t, workspaceRoot]);

  useEffect(() => {
    if (noWorkspace) return;
    const current = normalizeWritePath(browsePath);
    if (!selectedPath) {
      if (visibleEntries.every((entry) => entry.isDir)) return;
      const firstFile = visibleEntries.find((entry) => !entry.isDir);
      if (firstFile) onSelectFile(firstFile.path);
      return;
    }
    const selected = normalizeWritePath(selectedPath);
    if (pathsEqual(parentWritePath(selected), current)) return;
    const firstFile = visibleEntries.find((entry) => !entry.isDir);
    onSelectFile(firstFile?.path ?? "");
  }, [browsePath, noWorkspace, onSelectFile, selectedPath, visibleEntries]);

  const createDraft = async () => {
    const stamp = new Date().toISOString().slice(0, 10);
    const name = `untitled-${stamp}.md`;
    let path: string;
    if (noWorkspace) {
      const picked = await app.PickSaveFilePath(name);
      if (!picked?.trim()) return;
      path = picked.trim();
    } else {
      path = `${normalizeWritePath(browsePath)}/${name}`;
    }
    await app.WriteWriteFile(path, `# ${t("write.defaultDocTitle")}\n\n`);
    await reload();
    onFilesChanged?.();
    onSelectFile(path);
  };

  const requestSelect = async (path: string, isDir: boolean) => {
    if (isDir) {
      setBrowsePath(path);
      return;
    }
    if (dirty && selectedPath && !pathsEqual(path, selectedPath)) {
      const ok = await confirmAction({ title: t("write.unsavedLeaveTitle"), message: t("write.unsavedLeave") });
      if (!ok) return;
    }
    onSelectFile(path);
  };

  const deleteDraft = async (path: string) => {
    const ok = await confirmAction({
      title: t("write.sidebar.confirmDeleteTitle"),
      message: t("write.sidebar.confirmDelete", { name: baseName(path) }),
      destructive: true,
    });
    if (!ok) return;
    await app.DeleteWriteFile(path);
    await reload();
    onFilesChanged?.();
    if (selectedPath && pathsEqual(selectedPath, path)) onSelectFile("");
  };

  const renameDraft = async (path: string) => {
    const nextName = window.prompt(t("write.sidebar.renamePrompt"), baseName(path));
    if (!nextName?.trim() || nextName.trim() === baseName(path)) return;
    const nextPath = `${normalizeWritePath(browsePath)}/${nextName.trim()}`;
    await app.RenameWriteFile(path, nextPath);
    await reload();
    onFilesChanged?.();
    if (selectedPath && pathsEqual(selectedPath, path)) onSelectFile(nextPath);
  };

  const switchWorkspaceRoot = async (nextRoot: string) => {
    if (nextRoot === workspaceRoot) return;
    if (dirty) {
      const ok = await confirmAction({ title: t("write.unsavedLeaveTitle"), message: t("write.unsavedLeave") });
      if (!ok) return;
    }
    recordRecentWorkspace(nextRoot);
    onWorkspaceChange?.(nextRoot);
  };

  const handleWorkspaceSelect = async (value: string) => {
    setWorkspaceMenuOpen(false);
    if (value === PICK_FILE_VALUE) {
      const picked = await onPickFile?.();
      if (!picked) return;
      onSelectFile(picked);
      return;
    }
    if (value === PICK_WORKSPACE_VALUE) {
      const picked = await onPickWorkspace?.();
      if (!picked) return;
      switchWorkspaceRoot(picked);
      return;
    }
    switchWorkspaceRoot(value);
  };

  const goUp = () => {
    const root = normalizeWritePath(workspaceRoot);
    const current = normalizeWritePath(browsePath);
    if (pathsEqual(current, root)) return;
    const parent = parentWritePath(current);
    setBrowsePath(parent || root);
  };

  const currentFolderName = noWorkspace ? t("write.sidebar.noFolder") : baseName(workspaceRoot) || workspaceRoot;
  const rootPath = normalizeWritePath(workspaceRoot);
  const currentBrowsePath = normalizeWritePath(browsePath);
  const canGoUp = !noWorkspace && !pathsEqual(currentBrowsePath, rootPath);

  return (
    <aside className="write-sidebar write-studio__sidebar">
      <div className="write-sidebar__head write-studio__sidebar-head">
        <div className="write-sidebar__title">{t("write.sidebar.title")}</div>
        <button
          type="button"
          className="write-sidebar__icon write-studio__icon-btn"
          disabled={busy}
          onClick={() => void reload()}
          aria-label={t("write.sidebar.refreshFiles")}
        >
          <RefreshCw size={14} className={busy ? "dock-panel__spin" : undefined} />
        </button>
      </div>
      <p className="write-studio__skill-hint">{t("write.skillHint")}</p>

      <div className="studio-select">
        <span className="studio-select__label">{t("write.sidebar.location")}</span>
        <button
          ref={workspaceAnchorRef}
          type="button"
          className={`studio-select__trigger${workspaceMenuOpen ? " studio-select__trigger--open" : ""}`}
          onClick={() => setWorkspaceMenuOpen((open) => !open)}
        >
          <Folder size={14} className="studio-select__trigger-icon" />
          <span className="studio-select__trigger-value">{currentFolderName}</span>
          <ChevronDown size={13} className="studio-select__trigger-caret" />
        </button>
        <AnchoredPopover
          open={workspaceMenuOpen}
          anchorRef={workspaceAnchorRef}
          onClose={closeWorkspaceMenu}
          className="studio-select__popover"
          align="start"
          placement="bottom"
        >
          <div className="studio-select__menu">
            <div className="studio-select__menu-label">{t("write.sidebar.location")}</div>
            <button
              type="button"
              className={`studio-select__item${noWorkspace ? " studio-select__item--active" : ""}`}
              onClick={() => void handleWorkspaceSelect(NO_WORKSPACE_VALUE)}
            >
              <FolderX size={14} />
              <span>{t("write.sidebar.noFolderBind")}</span>
              {noWorkspace ? <Check size={14} /> : null}
            </button>
            {!noWorkspace ? (
              <button
                type="button"
                className="studio-select__item studio-select__item--active"
                onClick={() => setWorkspaceMenuOpen(false)}
              >
                <Folder size={14} />
                <span>{currentFolderName}</span>
              </button>
            ) : null}
            <button
              type="button"
              className="studio-select__item"
              onClick={() => void handleWorkspaceSelect(PICK_WORKSPACE_VALUE)}
            >
              <FolderOpen size={14} />
              <span>{t("write.sidebar.chooseFolder")}</span>
            </button>
            <button
              type="button"
              className="studio-select__item"
              onClick={() => void handleWorkspaceSelect(PICK_FILE_VALUE)}
            >
              <FileText size={14} />
              <span>{t("write.sidebar.chooseFile")}</span>
            </button>

            {recentWorkspaces.length > 0 ? (
              <>
                <div className="studio-select__menu-divider" />
                <div className="studio-select__menu-label">{t("write.sidebar.recentFolders")}</div>
                {recentWorkspaces.map((root) => (
                  <button
                    key={root}
                    type="button"
                    className="studio-select__item"
                    onClick={() => void handleWorkspaceSelect(root)}
                  >
                    <Folder size={14} />
                    <span>{baseName(root) || root}</span>
                  </button>
                ))}
              </>
            ) : null}
          </div>
        </AnchoredPopover>
      </div>

      {!noWorkspace ? (
      <nav className="write-studio__breadcrumb" aria-label={t("write.sidebar.currentFolder")}>
        {breadcrumbParts.map((part, index) => (
          <span key={part.path} className="write-studio__breadcrumb-part">
            {index > 0 ? <span className="write-studio__breadcrumb-sep">/</span> : null}
            <button type="button" onClick={() => setBrowsePath(part.path)}>
              {part.label}
            </button>
          </span>
        ))}
      </nav>
      ) : null}

      {canGoUp ? (
        <button type="button" className="write-studio__back-btn" onClick={goUp}>
          <ChevronLeft size={14} />
          <span>{t("write.sidebar.goUp")}</span>
        </button>
      ) : null}

      <button type="button" className="write-studio__new-btn" onClick={() => void createDraft()}>
        <Plus size={15} strokeWidth={2} />
        <span>{t("write.sidebar.newDraft")}</span>
      </button>

      <WriteFileListPane browsePath={browsePath}>
        <div className="write-sidebar__list write-studio__file-list">
        {visibleEntries.map((entry) =>
          entry.isDir ? (
            <button
              key={entry.path}
              type="button"
              className="write-sidebar__file write-studio__file-btn write-studio__file-btn--dir"
              onClick={() => requestSelect(entry.path, true)}
            >
              <Folder size={14} className="write-studio__file-ico" />
              <Tooltip label={entry.name}>
                <span className="write-studio__file-name">{entry.name}</span>
              </Tooltip>
            </button>
          ) : (
            <div
              key={entry.path}
              className={`write-studio__file-row${selectedPath && pathsEqual(selectedPath, entry.path) ? " write-studio__file-row--active" : ""}`}
            >
              <button
                type="button"
                className="write-sidebar__file write-studio__file-btn"
                onClick={() => requestSelect(entry.path, false)}
              >
                <FileTypeIcon name={entry.name} isDir={false} className="write-studio__file-ico" />
                <span className="write-studio__file-copy">
                  <Tooltip label={entry.name}>
                    <span className="write-studio__file-name">{entry.name}</span>
                  </Tooltip>
                  {entry.modTime ? <span className="write-studio__file-time">{formatModTime(entry.modTime)}</span> : null}
                </span>
              </button>
              <Tooltip label={t("write.sidebar.renameDraft")}>
                <button
                  type="button"
                  className="write-studio__file-delete"
                  aria-label={t("write.sidebar.renameDraft")}
                  onClick={() => void renameDraft(entry.path)}
                >
                  <Pencil size={13} />
                </button>
              </Tooltip>
              <Tooltip label={t("write.sidebar.deleteDraft")}>
                <button
                  type="button"
                  className="write-studio__file-delete"
                  aria-label={t("write.sidebar.deleteDraft")}
                  onClick={() => void deleteDraft(entry.path)}
                >
                  <Trash2 size={13} />
                </button>
              </Tooltip>
            </div>
          ),
        )}
        {!visibleEntries.length ? (
          <div className="write-sidebar__empty">
            <Folder size={16} />
            <span>{noWorkspace ? t("write.sidebar.noFolderEmpty") : canGoUp ? t("write.sidebar.emptyFolder") : t("write.sidebar.empty")}</span>
            {noWorkspace ? (
              <button type="button" className="btn btn--primary btn--small write-sidebar__empty-btn" onClick={() => void createDraft()}>
                {t("write.sidebar.newDraft")}
              </button>
            ) : onPickWorkspace || onPickFile ? (
              <div className="write-sidebar__empty-actions">
                {onPickFile ? (
                  <button type="button" className="btn btn--primary btn--small write-sidebar__empty-btn" onClick={() => void onPickFile()}>
                    {t("write.sidebar.chooseFile")}
                  </button>
                ) : null}
                {onPickWorkspace ? (
                  <button type="button" className="btn btn--ghost btn--small write-sidebar__empty-btn" onClick={() => void onPickWorkspace()}>
                    {t("write.sidebar.chooseFolder")}
                  </button>
                ) : null}
              </div>
            ) : null}
          </div>
        ) : null}
        </div>
      </WriteFileListPane>
    </aside>
  );
}
