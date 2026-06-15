import { useRef, useState } from "react";
import { ArrowLeft, Maximize2, Minimize2, Monitor, Plus, SquareTerminal, X } from "lucide-react";
import type { PreviewMode } from "../lib/dockHubs";
import { useT } from "../lib/i18n";
import type { BrowserTab } from "../lib/useBrowserPanel";
import type { ToolFileDiff } from "../lib/tools";
import { AnchoredPopover } from "./AnchoredPopover";
import { FilePreviewPanel } from "./FilePreviewPanel";
import { PagePreviewPanel } from "./PagePreviewPanel";
import { BrowserPanel } from "./BrowserPanel";
import { PreviewTerminalPane } from "./PreviewTerminalPane";
import type { TerminalTab } from "./TerminalPanel";
import { Tooltip } from "./Tooltip";

export interface UnifiedPreviewPanelProps {
  mode: PreviewMode;
  onClose: () => void;
  onAddTerminal: () => void;
  onAddBrowser: () => void;
  filePath?: string | null;
  fileDiff?: ToolFileDiff | null;
  onCloseFile?: () => void;
  onAddFileToChat?: (text: string) => void;
  pagePath?: string | null;
  onPagePathChange?: (path: string) => void;
  refreshKey?: number;
  workspaceRoot?: string;
  browserTabs?: BrowserTab[];
  activeBrowserTabId?: string | null;
  onBrowserTabChange?: (id: string) => void;
  onCloseBrowserTab?: (id: string) => void;
  onNewBrowserTab?: () => void;
  onBrowserTabUrlChange?: (id: string, url: string, title?: string) => void;
  terminalTabs?: TerminalTab[];
  activeTerminalId?: string | null;
  onTerminalTabChange?: (id: string) => void;
  onNewTerminal?: () => void;
  onCloseTerminalTab?: (id: string, index: number) => void;
  fileTreePreviewContext?: boolean;
  fileTreePreviewExpanded?: boolean;
  onExpandFileTreePreview?: () => void;
  onCollapseFileTreePreview?: () => void;
  onBackToFileTree?: () => void;
}

export function UnifiedPreviewPanel({
  mode,
  onClose,
  onAddTerminal,
  onAddBrowser,
  filePath,
  fileDiff,
  onCloseFile,
  onAddFileToChat,
  pagePath,
  onPagePathChange,
  refreshKey = 0,
  workspaceRoot,
  browserTabs = [],
  activeBrowserTabId = null,
  onBrowserTabChange,
  onCloseBrowserTab,
  onNewBrowserTab,
  onBrowserTabUrlChange,
  terminalTabs = [],
  activeTerminalId = null,
  onTerminalTabChange,
  onNewTerminal,
  onCloseTerminalTab,
  fileTreePreviewContext = false,
  fileTreePreviewExpanded = false,
  onExpandFileTreePreview,
  onCollapseFileTreePreview,
  onBackToFileTree,
}: UnifiedPreviewPanelProps) {
  const t = useT();
  const addAnchorRef = useRef<HTMLButtonElement | null>(null);
  const [addMenuOpen, setAddMenuOpen] = useState(false);
  const showFileTreeControls = (mode === "file" || mode === "page") && fileTreePreviewContext;
  const showFileTreeExpandedControls = showFileTreeControls && fileTreePreviewExpanded;

  return (
    <aside className="unified-preview" aria-label={t("previewHub.title")}>
      <header className="unified-preview__head unified-preview__head--minimal wails-no-drag">
        {showFileTreeExpandedControls ? (
          <Tooltip label={t("previewHub.backToFiles")}>
            <button
              type="button"
              className="unified-preview__iconbtn"
              onClick={onBackToFileTree}
              aria-label={t("previewHub.backToFiles")}
            >
              <ArrowLeft size={14} />
            </button>
          </Tooltip>
        ) : (
          <Tooltip label={t("previewHub.add")}>
            <button
              ref={addAnchorRef}
              type="button"
              className="unified-preview__iconbtn"
              onClick={() => setAddMenuOpen((open) => !open)}
              aria-label={t("previewHub.add")}
              aria-expanded={addMenuOpen}
              aria-haspopup="menu"
            >
              <Plus size={14} />
            </button>
          </Tooltip>
        )}
        <span className="unified-preview__head-spacer" aria-hidden="true" />
        {showFileTreeControls ? (
          <Tooltip label={t(fileTreePreviewExpanded ? "previewHub.collapse" : "previewHub.expand")}>
            <button
              type="button"
              className="unified-preview__iconbtn"
              onClick={fileTreePreviewExpanded ? onCollapseFileTreePreview : onExpandFileTreePreview}
              aria-label={t(fileTreePreviewExpanded ? "previewHub.collapse" : "previewHub.expand")}
              aria-pressed={fileTreePreviewExpanded}
            >
              {fileTreePreviewExpanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
            </button>
          </Tooltip>
        ) : null}
        <Tooltip label={t("previewHub.close")}>
          <button type="button" className="unified-preview__iconbtn" onClick={onClose} aria-label={t("previewHub.close")}>
            <X size={14} />
          </button>
        </Tooltip>
      </header>

      <AnchoredPopover
        open={addMenuOpen && !showFileTreeExpandedControls}
        anchorRef={addAnchorRef}
        onClose={() => setAddMenuOpen(false)}
        className="dock-hub-menu"
        align="start"
        placement="bottom"
        offset={6}
      >
        <div className="dock-hub-menu__list" role="menu">
          <button
            type="button"
            role="menuitem"
            className="dock-hub-menu__item"
            onClick={() => {
              setAddMenuOpen(false);
              onAddTerminal();
            }}
          >
            <span className="dock-hub-menu__item-main">
              <SquareTerminal size={14} />
              <span>{t("previewHub.terminal")}</span>
            </span>
          </button>
          <button
            type="button"
            role="menuitem"
            className="dock-hub-menu__item"
            onClick={() => {
              setAddMenuOpen(false);
              onAddBrowser();
            }}
          >
            <span className="dock-hub-menu__item-main">
              <Monitor size={14} />
              <span>{t("previewHub.browser")}</span>
            </span>
          </button>
        </div>
      </AnchoredPopover>

      <div className="unified-preview__body">
        {mode === "file" && (
          filePath ? (
            <FilePreviewPanel
              path={filePath}
              diff={fileDiff}
              embedded
              onClose={onCloseFile ?? onClose}
              onAddToChat={onAddFileToChat}
            />
          ) : (
            <div className="unified-preview__empty">
              <p>{t("previewHub.fileEmpty")}</p>
              <small>{t("previewHub.fileEmptyHint")}</small>
            </div>
          )
        )}

        {mode === "page" && (
          <PagePreviewPanel
            embedded
            pagePath={pagePath}
            onPagePathChange={onPagePathChange}
            refreshKey={refreshKey}
            workspaceRoot={workspaceRoot}
          />
        )}

        {mode === "terminal" && (
          <PreviewTerminalPane
            tabs={terminalTabs}
            activeId={activeTerminalId ?? ""}
            onActiveChange={(id) => onTerminalTabChange?.(id)}
            onNewTerminal={() => onNewTerminal?.()}
            onCloseTab={(id, index) => onCloseTerminalTab?.(id, index)}
          />
        )}

        {mode === "browser" && (
          <BrowserPanel
            embedded
            tabs={browserTabs}
            activeId={activeBrowserTabId}
            onActiveChange={(id) => onBrowserTabChange?.(id)}
            onCloseTab={(id) => onCloseBrowserTab?.(id)}
            onNewTab={() => onNewBrowserTab?.()}
            onTabUrlChange={(id, url, title) => onBrowserTabUrlChange?.(id, url, title)}
            refreshKey={refreshKey}
            workspaceRoot={workspaceRoot}
          />
        )}
      </div>
    </aside>
  );
}
