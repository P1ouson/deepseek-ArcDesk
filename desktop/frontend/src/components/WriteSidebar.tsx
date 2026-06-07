import { useCallback, useEffect, useMemo, useState } from "react";
import { FileText, Folder, Plus, RefreshCw } from "lucide-react";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { FileEntry } from "../lib/types";

export interface WriteSidebarProps {
  workspaceRoot: string;
  selectedPath?: string;
  onSelectFile: (path: string) => void;
  onWorkspaceChange?: (root: string) => void;
  onFilesChanged?: () => void;
}

function baseName(path: string): string {
  return path.replace(/[/\\]+$/, "").split(/[/\\]/).filter(Boolean).pop() ?? path;
}

export function WriteSidebar({ workspaceRoot, selectedPath, onSelectFile, onWorkspaceChange, onFilesChanged }: WriteSidebarProps) {
  const t = useT();
  const [workspaces, setWorkspaces] = useState<string[]>([]);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async () => {
    setBusy(true);
    try {
      const [roots, entries] = await Promise.all([
        app.ListWriteWorkspaces().catch(() => [] as string[]),
        app.ListWriteFiles(workspaceRoot).catch(() => [] as FileEntry[]),
      ]);
      setWorkspaces(roots.length ? roots : [workspaceRoot]);
      setFiles(entries.filter((entry) => !entry.isDir));
    } finally {
      setBusy(false);
    }
  }, [workspaceRoot]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const sortedFiles = useMemo(
    () => [...files].sort((a, b) => a.name.localeCompare(b.name)),
    [files],
  );

  const createDraft = async () => {
    const name = `draft-${Date.now()}.md`;
    const path = `${workspaceRoot.replace(/[/\\]+$/, "")}/${name}`;
    await app.WriteWriteFile(path, `# ${name.replace(/\.md$/i, "")}\n\n`);
    await reload();
    onFilesChanged?.();
    onSelectFile(path);
  };

  return (
    <aside className="write-sidebar">
      <div className="write-sidebar__head">
        <div className="write-sidebar__title">{t("write.sidebar.title")}</div>
        <button type="button" className="write-sidebar__icon" disabled={busy} onClick={() => void reload()} aria-label={t("write.sidebar.refreshFiles")}>
          <RefreshCw size={14} />
        </button>
      </div>

      <label className="write-sidebar__field">
        <span>{t("write.sidebar.root")}</span>
        <select
          value={workspaceRoot}
          onChange={(e) => {
            onWorkspaceChange?.(e.target.value);
          }}
        >
          {(workspaces.length ? workspaces : [workspaceRoot]).map((root) => (
            <option key={root} value={root}>
              {baseName(root) || root}
            </option>
          ))}
        </select>
      </label>

      <button type="button" className="write-sidebar__new" onClick={() => void createDraft()}>
        <Plus size={15} />
        {t("write.sidebar.newDraft")}
      </button>

      <div className="write-sidebar__list">
        {sortedFiles.map((file) => (
          <button
            key={file.path}
            type="button"
            className={`write-sidebar__file${selectedPath === file.path ? " write-sidebar__file--active" : ""}`}
            onClick={() => onSelectFile(file.path)}
          >
            <FileText size={14} />
            <span>{file.name}</span>
          </button>
        ))}
        {!sortedFiles.length ? (
          <div className="write-sidebar__empty">
            <Folder size={16} />
            <span>{t("write.sidebar.empty")}</span>
          </div>
        ) : null}
      </div>
    </aside>
  );
}
