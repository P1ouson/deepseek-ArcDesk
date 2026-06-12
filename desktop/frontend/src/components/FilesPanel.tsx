import { useCallback, useEffect, useRef, useState, type DragEvent as ReactDragEvent, type MouseEvent as ReactMouseEvent } from "react";
import {
  ChevronRight,
  Copy,
  Eye,
  FileText,
  FolderOpen,
  MessageSquarePlus,
  RefreshCw,
} from "lucide-react";
import { FileTypeIcon } from "./FileTypeIcon";
import { app } from "../lib/bridge";
import { IPC_LIST_DIR_TIMEOUT_MS, withIPCTimeout } from "../lib/ipc";
import { useT } from "../lib/i18n";
import type { DictKey } from "../locales/en";
import { parentDirs, shortCwd } from "../lib/workspaceFilePreview";
import { addWorkspaceFileContentToChat } from "../lib/workspaceAddToChat";
import type { DirEntry } from "../lib/types";
import { formatWorkspaceReference } from "../lib/workspaceDrag";
import { startWorkspaceDrag } from "../lib/workspaceDockActions";
import { DockPanelHeader } from "./dock/DockPanelHeader";
import { DockEmptyState } from "./dock/DockEmptyState";
import { isPreviewablePagePath } from "../lib/previewPage";
import { useDismissOverlay } from "../lib/useDismissOverlay";
import { ContextMenu, contextMenuPointFromEvent, type ContextMenuItem, type ContextMenuPoint } from "./ContextMenu";
import { MotionUnfold } from "./MotionUnfold";
import { FloatingMenu, FloatingMenuItems } from "./FloatingMenu";

interface FilesPanelProps {
  cwd?: string;
  refreshKey?: number;
  activeFilePath?: string | null;
  onOpenFile: (path: string) => void;
  onPreviewPage?: (path: string) => void;
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

export function FilesPanel({ cwd, refreshKey, activeFilePath, onOpenFile, onPreviewPage, onAddToChat }: FilesPanelProps) {
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
    const entries = await withIPCTimeout(app.ListDir(dir), IPC_LIST_DIR_TIMEOUT_MS, "ListDir").catch(() => []);
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

  useDismissOverlay(Boolean(treeMenu), () => setTreeMenu(null), { mode: "click" });

  const startTreeDrag = (event: ReactDragEvent<HTMLElement>, path: string, isDir: boolean) => {
    startWorkspaceDrag(event, path, isDir);
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

  const openPreviewPage = () => {
    if (!treeMenu || treeMenu.isDir) return;
    onPreviewPage?.(treeMenu.path);
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
          <DockEmptyState title={t("workspace.loading")} />
        </ul>
      );
    }
    if (entries.length === 0 && isRoot) {
      return (
        <ul className="dock-panel__list files-panel__tree">
          <DockEmptyState title={t("files.empty")} hint={t("files.emptyHint")} />
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
                  {entry.isDir ? <ChevronRight size={12} strokeWidth={1.75} /> : null}
                </span>
                <FileTypeIcon name={entry.name} isDir={entry.isDir} isOpen={isOpen} />
                <span className="files-panel__label">{entry.name}</span>
              </button>
              {entry.isDir ? (
                <MotionUnfold open={isOpen}>
                  {renderBranch(path)}
                </MotionUnfold>
              ) : null}
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
      <DockPanelHeader
        title={t("rightDock.tab.files")}
        cwd={cwd}
        cwdLabel={shortCwd(cwd) || t("workspace.title")}
        refreshLabel={t("workspace.refreshTree")}
        refreshing={refreshing}
        onRefresh={() => void refreshTree()}
      />

      {renderBranch("", true)}

      {treeMenu && (
        <FloatingMenu
          x={treeMenu.x}
          y={treeMenu.y}
          estimatedHeight={treeMenu.isDir ? MENU_DIR_HEIGHT : MENU_FILE_HEIGHT}
          className="workspace-tree-menu"
          onClose={() => setTreeMenu(null)}
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
                    ...(isPreviewablePagePath(treeMenu.path)
                      ? [
                          {
                            icon: <Eye size={14} />,
                            label: t("pagePreview.open"),
                            onSelect: openPreviewPage,
                          },
                        ]
                      : []),
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
