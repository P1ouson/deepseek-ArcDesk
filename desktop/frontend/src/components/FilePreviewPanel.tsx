import { useCallback, useEffect, useRef, useState, type MouseEvent as ReactMouseEvent } from "react";
import { Copy, FileText, FolderOpen, Maximize2, MessageSquarePlus, Minimize2, X } from "lucide-react";
import { app } from "../lib/bridge";
import { toErrorMessage } from "../lib/errors";
import { useT } from "../lib/i18n";
import type { DictKey } from "../locales/en";
import {
  basename,
  formatSelectionReference,
  isImagePath,
  isSvgPath,
  languageFor,
  parentPath,
} from "../lib/workspaceFilePreview";
import type { FilePreview } from "../lib/types";
import { formatWorkspaceReference } from "../lib/workspaceDrag";
import type { ToolFileDiff } from "../lib/tools";
import { toolDiffStat } from "../lib/tools";
import { CodeViewer } from "./CodeViewer";
import { UnifiedDiffView } from "./DiffView";
import { FloatingMenu, FloatingMenuItems } from "./FloatingMenu";
import { Markdown } from "./Markdown";
import { Tooltip } from "./Tooltip";

export interface FilePreviewPanelProps {
  path: string;
  diff?: ToolFileDiff | null;
  expanded: boolean;
  onToggleExpanded: () => void;
  onClose: () => void;
  onAddToChat?: (text: string) => void;
}

function revealInFileManagerLabelKey(platform: string): DictKey {
  if (platform === "darwin") return "projectTree.revealInFinder";
  if (platform === "windows") return "projectTree.revealInExplorer";
  return "projectTree.revealInFileManager";
}

export function FilePreviewPanel({ path, diff, expanded, onToggleExpanded, onClose, onAddToChat }: FilePreviewPanelProps) {
  const t = useT();
  const bodyRef = useRef<HTMLDivElement>(null);
  const [platform, setPlatform] = useState("");
  const [preview, setPreview] = useState<FilePreview | null>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadErr, setLoadErr] = useState<string | null>(null);
  const [selectionMenu, setSelectionMenu] = useState<{ x: number; y: number; text: string } | null>(null);

  useEffect(() => {
    let cancelled = false;
    void app.Platform().then((value) => {
      if (!cancelled) setPlatform(value);
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setLoadErr(null);
    setPreview(null);
    setImageUrl(null);
    setSelectionMenu(null);

    if (diff?.diff) {
      setLoading(false);
      return () => {
        cancelled = true;
      };
    }

    const load = async () => {
      try {
        if (isImagePath(path)) {
          const url = await app.ReadWorkspaceFileDataURL(path);
          if (cancelled) return;
          setImageUrl(url);
          return;
        }
        const next = await app.ReadFile(path);
        if (cancelled) return;
        setPreview(next);
        if (next.err) setLoadErr(next.err);
      } catch (err) {
        if (!cancelled) setLoadErr(toErrorMessage(err));
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [path, diff?.diff]);

  useEffect(() => {
    if (!selectionMenu) return;
    const close = () => setSelectionMenu(null);
    window.addEventListener("click", close);
    window.addEventListener("resize", close);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("resize", close);
    };
  }, [selectionMenu]);

  const copyPath = useCallback(async () => {
    try {
      await navigator.clipboard?.writeText(path);
    } catch {
      /* clipboard unavailable */
    }
  }, [path]);

  const addReference = useCallback(() => {
    onAddToChat?.(formatWorkspaceReference(path, false));
  }, [onAddToChat, path]);

  const openSelectionMenu = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (isImagePath(path)) return;
    const selection = window.getSelection()?.toString().trim();
    if (!selection) return;
    event.preventDefault();
    setSelectionMenu({ x: event.clientX, y: event.clientY, text: selection });
  };

  const addSelectionToChat = () => {
    if (!selectionMenu) return;
    onAddToChat?.(formatSelectionReference(path, selectionMenu.text));
    setSelectionMenu(null);
  };

  const isMarkdown = path.toLowerCase().endsWith(".md");
  const showImage = !diff?.diff && isImagePath(path) && imageUrl;
  const showSvg = !diff?.diff && isSvgPath(path) && preview && !preview.binary && !preview.err;
  const diffStat = diff ? toolDiffStat(diff) : "";

  return (
    <aside className={`file-preview-panel${expanded ? " file-preview-panel--expanded" : ""}`} aria-label={t("filePreview.title")}>
      <header className="file-preview-panel__head wails-no-drag">
        <div className="file-preview-panel__title">
          <FileText size={14} />
          <div className="file-preview-panel__title-text">
            <Tooltip label={path}>
              <span className="file-preview-panel__name">{basename(path)}</span>
            </Tooltip>
            {parentPath(path) && <span className="file-preview-panel__path">{parentPath(path)}</span>}
            {diffStat ? <span className="file-preview-panel__diff-stat">{diffStat}</span> : null}
          </div>
        </div>
        <div className="file-preview-panel__actions wails-no-drag">
          <Tooltip label={t("workspace.addFileReferenceToChat")}>
            <button type="button" className="file-preview-panel__iconbtn" onClick={addReference} aria-label={t("workspace.addFileReferenceToChat")}>
              <MessageSquarePlus size={14} />
            </button>
          </Tooltip>
          <Tooltip label={t(expanded ? "filePreview.collapse" : "filePreview.expand")}>
            <button
              type="button"
              className="file-preview-panel__iconbtn"
              onClick={onToggleExpanded}
              aria-label={t(expanded ? "filePreview.collapse" : "filePreview.expand")}
              aria-pressed={expanded}
            >
              {expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
            </button>
          </Tooltip>
          <Tooltip label={t(revealInFileManagerLabelKey(platform))}>
            <button type="button" className="file-preview-panel__iconbtn" onClick={() => void app.RevealWorkspacePath(path)} aria-label={t(revealInFileManagerLabelKey(platform))}>
              <FolderOpen size={14} />
            </button>
          </Tooltip>
          <span className="file-preview-panel__actions-spacer" aria-hidden="true" />
          <Tooltip label={t("projectTree.copyPath")}>
            <button type="button" className="file-preview-panel__iconbtn" onClick={() => void copyPath()} aria-label={t("projectTree.copyPath")}>
              <Copy size={14} />
            </button>
          </Tooltip>
          <Tooltip label={t("filePreview.close")}>
            <button type="button" className="file-preview-panel__iconbtn" onClick={onClose} aria-label={t("filePreview.close")}>
              <X size={14} />
            </button>
          </Tooltip>
        </div>
      </header>

      <div className="file-preview-panel__body" ref={bodyRef} onContextMenu={openSelectionMenu}>
        {diff?.diff ? (
          <UnifiedDiffView unified={diff.diff} language={languageFor(path)} maxHeight={undefined} />
        ) : loading ? (
          <div className="file-preview-panel__empty">{t("workspace.loading")}</div>
        ) : loadErr ? (
          <div className="file-preview-panel__empty">{loadErr}</div>
        ) : showImage ? (
          <div className="file-preview-panel__image-wrap">
            <img src={imageUrl!} alt={basename(path)} className="file-preview-panel__image" />
          </div>
        ) : preview?.binary ? (
          <div className="file-preview-panel__empty">{t("workspace.binary")}</div>
        ) : preview ? (
          <>
            {preview.truncated && <div className="file-preview-panel__note">{t("workspace.truncated")}</div>}
            {isMarkdown || showSvg ? (
              <Markdown text={preview.body} />
            ) : (
              <CodeViewer value={preview.body || " "} language={languageFor(path)} flat lineNumbers />
            )}
          </>
        ) : null}
      </div>

      {selectionMenu && (
        <FloatingMenu x={selectionMenu.x} y={selectionMenu.y} estimatedHeight={48} onClose={() => setSelectionMenu(null)}>
          <FloatingMenuItems
            items={[
              {
                icon: <MessageSquarePlus size={14} />,
                label: t("workspace.addSelectionToChat"),
                onSelect: addSelectionToChat,
              },
            ]}
          />
        </FloatingMenu>
      )}
    </aside>
  );
}
