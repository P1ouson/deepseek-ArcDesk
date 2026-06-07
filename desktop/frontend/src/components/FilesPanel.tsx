import { useCallback, useEffect, useRef, useState, type DragEvent as ReactDragEvent, type MouseEvent as ReactMouseEvent } from "react";
import {
  ChevronDown,
  ChevronRight,
  Copy,
  FileText,
  FolderOpen,
  MessageSquarePlus,
  RefreshCw,
} from "lucide-react";
import { FileTypeIcon } from "./FileTypeIcon";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { DictKey } from "../locales/en";
import { parentDirs, shortCwd } from "../lib/workspaceFilePreview";
import { addWorkspaceFileContentToChat } from "../lib/workspaceAddToChat";
import type { DirEntry } from "../lib/types";
import { formatWorkspaceReference, WORKSPACE_REF_DRAG_TYPE } from "../lib/workspaceDrag";
import { useDismissOnClickOutside } from "../lib/useDismissOnClickOutside";
import { ContextMenu, contextMenuPointFromEvent, type ContextMenuItem, type ContextMenuPoint } from "./ContextMenu";
import { FloatingMenu, FloatingMenuItems } from "./FloatingMenu";
import { Tooltip } from "./Tooltip";

interface FilesPanelProps {
  cwd?: string;
  refreshKey?: number;
  activeFilePath?: string | null;
  onOpenFile: (path: string) => void;
  onAddToChat?: (text: string) => void;
}

const MENU_DIR_HEIGHT = 120;
const MENU_FILE_HEIGHT = 168;

function entryPath(dir: string, entry: DirEntry): string {
  const prefix = dir === "" || dir.endsWith("/") ? dir : `${dir}/`;
  return prefix + entry.name + (entry.isDir ? "/" : "");
}

function revealInFileManagerLabelKey(platform: string): DictKey {
  if (platform === "darwin") return "projectTree.revealInFinder";
  if (platform === "windows") return "projectTree.revealInExplorer";
  return "projectTree.revealInFileManager";
}

export function FilesPanel({ cwd, refreshKey, activeFilePath, onOpenFile, onAddToChat }: FilesPanelProps) {
  const t = useT();
  const openDirsRef = useRef<Set<string>>(new Set([""]));
  const [platform, setPlatform] = useState("");
  const [entriesByDir, setEntriesByDir] = useState<Record<string, DirEntry[]>>({});
  const [openDirs, setOpenDirs] = useState<Set<string>>(() => new Set([""]));
  const [refreshing, setRefreshing] = useState(false);
  const [treeMenu, setTreeMenu] = useState<{ x: number; y: number; path: string; isDir: boolean } | null>(null);
  const [treeBlankMenuPoint, setTreeBlankMenuPoint] = useState<ContextMenuPoint | null>(null);

  useEffect(() => {
    openDirsRef.current = openDirs;
  }, [openDirs]);

  useEffect(() => {
    let cancelled = false;
    void app.Platform().then((value) => {
      if (!cancelled) setPlatform(value);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  const loadDir = useCallback(async (dir: string) => {
    const entries = await app.ListDir(dir).catch(() => []);
    setEntriesByDir((prev) => ({ ...prev, [dir]: entries ?? [] }));
  }, []);

  const openFile = useCallback(
    (path: string) => {
      onOpenFile(path);
      const dirs = parentDirs(path);
      setOpenDirs((prev) => new Set([...Array.from(prev), ...dirs]));
      dirs.forEach((dir) => {
        if (!entriesByDir[dir]) void loadDir(dir);
      });
    },
    [entriesByDir, loadDir, onOpenFile],
  );

  const toggleDir = useCallback(
    (dir: string) => {
      setOpenDirs((prev) => {
        const next = new Set(prev);
        if (next.has(dir)) next.delete(dir);
        else {
          next.add(dir);
          if (!entriesByDir[dir]) void loadDir(dir);
        }
        return next;
      });
    },
    [entriesByDir, loadDir],
  );

  const copyWorkspacePath = useCallback(async (rel: string) => {
    try {
      await navigator.clipboard?.writeText(rel);
    } catch {
      /* clipboard unavailable */
    }
  }, []);

  const refreshTree = useCallback(async () => {
    setTreeBlankMenuPoint(null);
    setTreeMenu(null);
    setRefreshing(true);
    const dirs = Array.from(openDirsRef.current);
    setEntriesByDir({});
    try {
      await Promise.all(dirs.map((dir) => loadDir(dir)));
    } finally {
      setRefreshing(false);
    }
  }, [loadDir]);

  useEffect(() => {
    setEntriesByDir({});
    setOpenDirs(new Set([""]));
    setTreeMenu(null);
    void loadDir("");
  }, [cwd, loadDir]);

  useEffect(() => {
    if (!refreshKey) return;
    void refreshTree();
  }, [refreshKey, refreshTree]);

  useEffect(() => {
    if (!activeFilePath) return;
    const dirs = parentDirs(activeFilePath);
    setOpenDirs((prev) => new Set([...Array.from(prev), ...dirs]));
    dirs.forEach((dir) => {
      if (!entriesByDir[dir]) void loadDir(dir);
    });
  }, [activeFilePath, entriesByDir, loadDir]);

  useDismissOnClickOutside(Boolean(treeMenu), () => setTreeMenu(null));

  const startTreeDrag = (event: ReactDragEvent<HTMLElement>, path: string, isDir: boolean) => {
    const ref = formatWorkspaceReference(path, isDir);
    event.dataTransfer.effectAllowed = "copy";
    event.dataTransfer.setData(WORKSPACE_REF_DRAG_TYPE, JSON.stringify({ path, isDir }));
    event.dataTransfer.setData("text/plain", ref);
  };

  const openTreeMenu = (event: ReactMouseEvent<HTMLElement>, path: string, isDir: boolean) => {
    event.preventDefault();
    event.stopPropagation();
    setTreeBlankMenuPoint(null);
    setTreeMenu({ x: event.clientX, y: event.clientY, path, isDir });
  };

  const openTreeBlankMenu = (event: ReactMouseEvent<HTMLUListElement>) => {
    const target = event.target as HTMLElement | null;
    if (target?.closest(".files-panel__node,button,input,textarea,select")) return;
    event.preventDefault();
    event.stopPropagation();
    setTreeMenu(null);
    setTreeBlankMenuPoint(contextMenuPointFromEvent(event));
  };

  const addTreeReferenceToChat = () => {
    if (!treeMenu) return;
    onAddToChat?.(formatWorkspaceReference(treeMenu.path, treeMenu.isDir));
    setTreeMenu(null);
  };

  const addTreeFileToChat = async () => {
    if (!treeMenu || treeMenu.isDir) return;
    const target = treeMenu;
    setTreeMenu(null);
    if (!onAddToChat) return;
    await addWorkspaceFileContentToChat(target.path, onAddToChat, t("workspace.truncated"));
  };

  const revealTreePath = () => {
    if (!treeMenu) return;
    void app.RevealWorkspacePath(treeMenu.path);
    setTreeMenu(null);
  };

  const copyTreePath = () => {
    if (!treeMenu) return;
    void copyWorkspacePath(treeMenu.path);
    setTreeMenu(null);
  };

  const renderBranch = (dir: string, isRoot = false): JSX.Element => {
    const entries = entriesByDir[dir];
    if (entries === undefined) {
      if (!isRoot) return <></>;
      return (
        <ul className="dock-panel__list files-panel__tree">
          <li className="dock-panel__empty">{t("workspace.loading")}</li>
        </ul>
      );
    }
    if (entries.length === 0 && isRoot) {
      return (
        <ul className="dock-panel__list files-panel__tree">
          <li className="dock-panel__empty">
            <span>{t("files.empty")}</span>
            <small>{t("files.emptyHint")}</small>
          </li>
        </ul>
      );
    }

    const listClass = isRoot ? "dock-panel__list files-panel__tree" : "files-panel__branch";

    return (
      <ul className={listClass} onContextMenu={isRoot ? openTreeBlankMenu : undefined}>
        {entries.map((entry) => {
          const path = entryPath(dir, entry);
          const isOpen = entry.isDir && openDirs.has(path);
          const active = !entry.isDir && activeFilePath === path;
          return (
            <li key={path} className="files-panel__item">
              <button
                type="button"
                className={`files-panel__node${entry.isDir ? " files-panel__node--dir" : ""}${active ? " files-panel__node--active" : ""}${isOpen ? " files-panel__node--open" : ""}`}
                draggable
                onDragStart={(event) => startTreeDrag(event, path, entry.isDir)}
                onClick={() => (entry.isDir ? toggleDir(path) : openFile(path))}
                onContextMenu={(event) => openTreeMenu(event, path, entry.isDir)}
              >
                <span className="files-panel__chevron" aria-hidden="true">
                  {entry.isDir ? isOpen ? <ChevronDown size={12} strokeWidth={1.75} /> : <ChevronRight size={12} strokeWidth={1.75} /> : null}
                </span>
                <FileTypeIcon name={entry.name} isDir={entry.isDir} isOpen={isOpen} />
                <span className="files-panel__label">{entry.name}</span>
              </button>
              {entry.isDir && isOpen ? renderBranch(path) : null}
            </li>
          );
        })}
      </ul>
    );
  };

  const treeBlankMenuItems: ContextMenuItem[] = [
    {
      key: "refresh-tree",
      icon: <RefreshCw size={13} />,
      label: t("workspace.refreshTree"),
      onSelect: () => void refreshTree(),
    },
  ];

  return (
    <div className="dock-panel files-panel">
      <header className="dock-panel__head">
        <div className="dock-panel__head-main">
          <h2 className="dock-panel__title">{t("rightDock.tab.files")}</h2>
          <Tooltip label={cwd ?? undefined}>
            <p className="dock-panel__meta">{shortCwd(cwd) || t("workspace.title")}</p>
          </Tooltip>
        </div>
        <Tooltip label={t("workspace.refreshTree")}>
          <button type="button" className="dock-panel__ghost" aria-label={t("workspace.refreshTree")} onClick={() => void refreshTree()}>
            <RefreshCw size={14} strokeWidth={1.75} className={refreshing ? "dock-panel__spin" : undefined} />
          </button>
        </Tooltip>
      </header>

      {renderBranch("", true)}

      {treeMenu && (
        <FloatingMenu
          x={treeMenu.x}
          y={treeMenu.y}
          estimatedHeight={treeMenu.isDir ? MENU_DIR_HEIGHT : MENU_FILE_HEIGHT}
          className="workspace-tree-menu"
        >
          <FloatingMenuItems
            items={[
              {
                icon: <MessageSquarePlus size={14} />,
                label: treeMenu.isDir ? t("workspace.addFolderReferenceToChat") : t("workspace.addFileReferenceToChat"),
                onSelect: addTreeReferenceToChat,
              },
              ...(treeMenu.isDir
                ? []
                : [
                    {
                      icon: <FileText size={14} />,
                      label: t("workspace.addFileContentToChat"),
                      onSelect: () => void addTreeFileToChat(),
                    },
                  ]),
              {
                icon: <FolderOpen size={14} />,
                label: t(revealInFileManagerLabelKey(platform)),
                onSelect: revealTreePath,
              },
              {
                icon: <Copy size={14} />,
                label: t("projectTree.copyPath"),
                onSelect: copyTreePath,
              },
            ]}
          />
        </FloatingMenu>
      )}

      <ContextMenu
        open={Boolean(treeBlankMenuPoint)}
        point={treeBlankMenuPoint}
        items={treeBlankMenuItems}
        minWidth={168}
        ariaLabel={t("workspace.treeMenu")}
        onClose={() => setTreeBlankMenuPoint(null)}
      />
    </div>
  );
}
