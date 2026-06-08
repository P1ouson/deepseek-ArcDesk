import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  ChevronDown,
  ChevronUp,
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
import { getLocale, useT } from "../lib/i18n";
import type { FileEntry } from "../lib/types";
import { getRecentWorkspacePaths, recordRecentWorkspace } from "../lib/workspaceRecents";
import { isNoWriteWorkspace, NO_WORKSPACE_VALUE } from "../lib/writeWorkspace";
import { closeStudioSelect, openStudioSelect } from "../lib/studioSelectRegistry";
import { AnchoredPopover } from "./AnchoredPopover";
import { Tooltip } from "./Tooltip";

const PICK_WORKSPACE_VALUE = "__pick_workspace__";

export interface WriteSidebarProps {
  workspaceRoot: string;
  selectedPath?: string;
  dirty?: boolean;
  onSelectFile: (path: string) => void;
  onWorkspaceChange?: (root: string) => void;
  onPickWorkspace?: () => Promise<string | undefined>;
  onFilesChanged?: () => void;
}

function baseName(path: string): string {
  return path.replace(/[/\\]+$/, "").split(/[/\\]/).filter(Boolean).pop() ?? path;
}

function normalizePath(path: string): string {
  return path.replace(/\\/g, "/").replace(/\/+$/, "");
}

function isDirectChild(entryPath: string, parentPath: string): boolean {
  const parent = normalizePath(parentPath);
  const entry = normalizePath(entryPath);
  if (entry === parent || !entry.startsWith(`${parent}/`)) return false;
  const rel = entry.slice(parent.length + 1);
  return rel.length > 0 && !rel.includes("/");
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
  onFilesChanged,
}: WriteSidebarProps) {
  const t = useT();
  const workspaceAnchorRef = useRef<HTMLButtonElement>(null);
  const [workspaceMenuOpen, setWorkspaceMenuOpen] = useState(false);
  const closeWorkspaceMenu = useCallback(() => setWorkspaceMenuOpen(false), []);
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [browsePath, setBrowsePath] = useState(workspaceRoot);
  const [busy, setBusy] = useState(false);
  const noWorkspace = isNoWriteWorkspace(workspaceRoot);

  const recentWorkspaces = useMemo(() => {
    const current = normalizePath(workspaceRoot);
    return getRecentWorkspacePaths()
      .filter((path) => normalizePath(path) !== current)
      .slice(0, 5);
  }, [workspaceRoot]);

  useEffect(() => {
    setBrowsePath(workspaceRoot);
  }, [workspaceRoot]);

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
      const listed = await app.ListWriteFiles(workspaceRoot).catch(() => [] as FileEntry[]);
      setEntries(listed);
    } finally {
      setBusy(false);
    }
  }, [noWorkspace, workspaceRoot]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const visibleEntries = useMemo(() => {
    const dirs = entries.filter((entry) => entry.isDir && isDirectChild(entry.path, browsePath));
    const files = entries.filter((entry) => !entry.isDir && isDirectChild(entry.path, browsePath));
    dirs.sort((a, b) => a.name.localeCompare(b.name));
    files.sort((a, b) => (b.modTime ?? 0) - (a.modTime ?? 0) || a.name.localeCompare(b.name));
    return [...dirs, ...files];
  }, [browsePath, entries]);

  const breadcrumbParts = useMemo(() => {
    const root = normalizePath(workspaceRoot);
    const current = normalizePath(browsePath);
    if (current === root) return [{ path: root, label: baseName(root) || t("write.sidebar.root") }];
    const rel = current.slice(root.length + 1);
    const parts = [{ path: root, label: baseName(root) || t("write.sidebar.root") }];
    let acc = root;
    for (const segment of rel.split("/").filter(Boolean)) {
      acc = `${acc}/${segment}`;
      parts.push({ path: acc, label: segment });
    }
    return parts;
  }, [browsePath, t, workspaceRoot]);

  useEffect(() => {
    if (selectedPath || visibleEntries.every((entry) => entry.isDir)) return;
    const firstFile = visibleEntries.find((entry) => !entry.isDir);
    if (firstFile) onSelectFile(firstFile.path);
  }, [onSelectFile, selectedPath, visibleEntries]);

  const createDraft = async () => {
    const stamp = new Date().toISOString().slice(0, 10);
    const name = `untitled-${stamp}.md`;
    let path: string;
    if (noWorkspace) {
      const picked = await app.PickSaveFilePath(name);
      if (!picked?.trim()) return;
      path = picked.trim();
    } else {
      path = `${normalizePath(browsePath)}/${name}`;
    }
    await app.WriteWriteFile(path, `# ${t("write.defaultDocTitle")}\n\n`);
    await reload();
    onFilesChanged?.();
    onSelectFile(path);
  };

  const requestSelect = (path: string, isDir: boolean) => {
    if (isDir) {
      setBrowsePath(path);
      return;
    }
    if (dirty && selectedPath && path !== selectedPath) {
      if (!window.confirm(t("write.unsavedLeave"))) return;
    }
    onSelectFile(path);
  };

  const deleteDraft = async (path: string) => {
    if (!window.confirm(t("write.sidebar.confirmDelete", { name: baseName(path) }))) return;
    await app.DeleteWriteFile(path);
    await reload();
    onFilesChanged?.();
    if (selectedPath === path) onSelectFile("");
  };

  const renameDraft = async (path: string) => {
    const nextName = window.prompt(t("write.sidebar.renamePrompt"), baseName(path));
    if (!nextName?.trim() || nextName.trim() === baseName(path)) return;
    const nextPath = `${normalizePath(browsePath)}/${nextName.trim()}`;
    await app.RenameWriteFile(path, nextPath);
    await reload();
    onFilesChanged?.();
    if (selectedPath === path) onSelectFile(nextPath);
  };

  const switchWorkspaceRoot = (nextRoot: string) => {
    if (nextRoot === workspaceRoot) return;
    if (dirty && !window.confirm(t("write.unsavedLeave"))) return;
    recordRecentWorkspace(nextRoot);
    onWorkspaceChange?.(nextRoot);
  };

  const handleWorkspaceSelect = async (value: string) => {
    setWorkspaceMenuOpen(false);
    if (value === PICK_WORKSPACE_VALUE) {
      const picked = await onPickWorkspace?.();
      if (!picked) return;
      switchWorkspaceRoot(picked);
      return;
    }
    switchWorkspaceRoot(value);
  };

  const goUp = () => {
    const root = normalizePath(workspaceRoot);
    const current = normalizePath(browsePath);
    if (current === root) return;
    const parent = current.slice(0, current.lastIndexOf("/"));
    setBrowsePath(parent || root);
  };

  const currentWorkspaceName = noWorkspace ? t("composer.noWorkspace") : baseName(workspaceRoot) || workspaceRoot;

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
        <span className="studio-select__label">{t("write.sidebar.workspace")}</span>
        <button
          ref={workspaceAnchorRef}
          type="button"
          className={`studio-select__trigger${workspaceMenuOpen ? " studio-select__trigger--open" : ""}`}
          onClick={() => setWorkspaceMenuOpen((open) => !open)}
        >
          <Folder size={14} className="studio-select__trigger-icon" />
          <span className="studio-select__trigger-value">{currentWorkspaceName}</span>
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
            <div className="studio-select__menu-label">{t("write.sidebar.workspace")}</div>
            <button
              type="button"
              className={`studio-select__item${noWorkspace ? " studio-select__item--active" : ""}`}
              onClick={() => void handleWorkspaceSelect(NO_WORKSPACE_VALUE)}
            >
              <FolderX size={14} />
              <span>{t("composer.useNoWorkspace")}</span>
              {noWorkspace ? <Check size={14} /> : null}
            </button>
            {!noWorkspace ? (
              <button
                type="button"
                className="studio-select__item studio-select__item--active"
                onClick={() => setWorkspaceMenuOpen(false)}
              >
                <Folder size={14} />
                <span>{currentWorkspaceName}</span>
              </button>
            ) : null}
            <button
              type="button"
              className="studio-select__item"
              onClick={() => void handleWorkspaceSelect(PICK_WORKSPACE_VALUE)}
            >
              <FolderOpen size={14} />
              <span>{t("write.sidebar.chooseWorkspace")}</span>
            </button>

            {recentWorkspaces.length > 0 ? (
              <>
                <div className="studio-select__menu-divider" />
                <div className="studio-select__menu-label">{t("write.sidebar.recentWorkspaces")}</div>
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
      <nav className="write-studio__breadcrumb" aria-label={t("write.sidebar.root")}>
        {breadcrumbParts.map((part, index) => (
          <span key={part.path} className="write-studio__breadcrumb-part">
            {index > 0 ? <span className="write-studio__breadcrumb-sep">/</span> : null}
            <button type="button" onClick={() => setBrowsePath(part.path)}>
              {part.label}
            </button>
          </span>
        ))}
        {normalizePath(browsePath) !== normalizePath(workspaceRoot) ? (
          <button type="button" className="write-studio__breadcrumb-up" onClick={goUp} aria-label={t("write.sidebar.goUp")}>
            <ChevronUp size={13} />
          </button>
        ) : null}
      </nav>
      ) : null}

      <button type="button" className="write-studio__new-btn" onClick={() => void createDraft()}>
        <Plus size={15} strokeWidth={2} />
        <span>{t("write.sidebar.newDraft")}</span>
      </button>

      <div className="write-sidebar__list write-studio__file-list">
        {visibleEntries.map((entry) =>
          entry.isDir ? (
            <button
              key={entry.path}
              type="button"
              className="write-sidebar__file write-studio__file-btn write-studio__file-btn--dir"
              onClick={() => requestSelect(entry.path, true)}
            >
              <Folder size={14} />
              <span className="write-studio__file-name">{entry.name}</span>
            </button>
          ) : (
            <div
              key={entry.path}
              className={`write-studio__file-row${selectedPath === entry.path ? " write-studio__file-row--active" : ""}`}
            >
              <button
                type="button"
                className="write-sidebar__file write-studio__file-btn"
                onClick={() => requestSelect(entry.path, false)}
              >
                <FileText size={14} />
                <span className="write-studio__file-name">{entry.name}</span>
                {entry.modTime ? <span className="write-studio__file-time">{formatModTime(entry.modTime)}</span> : null}
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
            <span>{noWorkspace ? t("write.sidebar.noWorkspaceEmpty") : t("write.sidebar.empty")}</span>
            {noWorkspace ? (
              <button type="button" className="write-studio__pick-folder-btn" onClick={() => void createDraft()}>
                {t("write.sidebar.newDraft")}
              </button>
            ) : onPickWorkspace ? (
              <button type="button" className="write-studio__pick-folder-btn" onClick={() => void onPickWorkspace()}>
                {t("write.sidebar.chooseWorkspace")}
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
    </aside>
  );
}
