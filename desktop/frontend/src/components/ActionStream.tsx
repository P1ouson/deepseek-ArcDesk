import { memo, useEffect, useRef, useState } from "react";
import { ChevronRight, Loader2 } from "lucide-react";
import { AnchoredPopover } from "./AnchoredPopover";
import { CodeViewer } from "./CodeViewer";
import { FileTypeIcon } from "./FileTypeIcon";
import {
  type ActionFileRef,
  type ActionSegment,
  type SegmentEntry,
  filesForTool,
  isWriteTool,
  subjectLabel,
  verbForThinking,
  verbForTool,
} from "../lib/actionStream";
import type { ToolItem } from "../lib/actionStream";
import type { ToolFileDiff } from "../lib/tools";
import { useT } from "../lib/i18n";

export interface ActionFileOpenRequest {
  path: string;
  diff?: ToolFileDiff;
}

function prettyJson(json: string): string {
  try {
    return JSON.stringify(JSON.parse(json), null, 2);
  } catch {
    return json;
  }
}

function ActionFileLinks({
  files,
  running,
  onOpenFile,
}: {
  files: ActionFileRef[];
  running: boolean;
  onOpenFile?: (req: ActionFileOpenRequest) => void;
}) {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const resolved = files.length > 0 ? files : [];

  if (resolved.length === 0) return null;

  const primary = resolved[0]!;
  const clickable = !running && !!onOpenFile;

  const openPrimary = () => {
    if (!clickable) return;
    onOpenFile?.({ path: primary.openPath });
  };

  return (
    <>
      <button
        ref={anchorRef}
        type="button"
        className={`action-row__file${clickable ? "" : " action-row__file--disabled"}`}
        disabled={!clickable}
        onClick={openPrimary}
        onMouseEnter={() => {
          if (resolved.length > 1) setOpen(true);
        }}
        onMouseLeave={() => {
          if (resolved.length > 1) setOpen(false);
        }}
        onFocus={() => {
          if (resolved.length > 1) setOpen(true);
        }}
        onBlur={() => setOpen(false)}
      >
        <span className="action-row__file-name">{primary.fileName}</span>
        {primary.relativePath ? (
          <span className="action-row__file-path">{primary.relativePath}</span>
        ) : null}
        {resolved.length > 1 ? <span className="action-row__file-more">…</span> : null}
      </button>

      {resolved.length > 1 && (
        <AnchoredPopover
          open={open}
          anchorRef={anchorRef}
          onClose={() => setOpen(false)}
          className="action-file-popover"
          align="start"
          placement="bottom"
          offset={6}
        >
          <ul className="action-file-popover__list" role="list">
            {resolved.map((file) => (
              <li key={file.openPath}>
                <button
                  type="button"
                  className="action-file-popover__item"
                  disabled={!clickable}
                  onClick={() => {
                    if (!clickable) return;
                    onOpenFile?.({ path: file.openPath });
                    setOpen(false);
                  }}
                >
                  <FileTypeIcon name={file.fileName} isDir={false} />
                  <span className="action-file-popover__copy">
                    <span className="action-file-popover__name">{file.fileName}</span>
                    {file.relativePath ? (
                      <span className="action-file-popover__path">{file.relativePath}</span>
                    ) : null}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </AnchoredPopover>
      )}
    </>
  );
}

const ActionToolRow = memo(function ActionToolRow({
  item,
  workspaceRoot,
  showCollapseToggle,
  collapsed,
  onToggleCollapse,
  onOpenFile,
}: {
  item: ToolItem;
  workspaceRoot: string;
  showCollapseToggle?: boolean;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
  onOpenFile?: (req: ActionFileOpenRequest) => void;
}) {
  const t = useT();
  const [detailOpen, setDetailOpen] = useState(false);
  const running = item.status === "running";
  const verb = verbForTool(item.name, item.status);
  const files = filesForTool(item, workspaceRoot);
  const subject = subjectLabel(item);
  const hasFileLinks = files.length > 0;
  const hasDetails = !!(item.output || item.error || (item.args && !hasFileLinks && item.isShell));
  const expandable = hasDetails && !running;

  const handleFileOpen = (req: ActionFileOpenRequest) => {
    if (running) return;
    if (isWriteTool(item.name) && item.fileDiff?.diff) {
      onOpenFile?.({ path: req.path, diff: item.fileDiff });
      return;
    }
    onOpenFile?.(req);
  };

  return (
    <div
      className={`action-row action-row--${item.status}${item.isShell ? " action-row--shell" : ""}${item.readOnly ? " action-row--readonly" : ""}`}
    >
      <div className="action-row__main">
        <span className="action-row__verb">{verb}</span>
        {hasFileLinks ? (
          <ActionFileLinks files={files} running={running} onOpenFile={handleFileOpen} />
        ) : subject ? (
          <span className="action-row__subject">{subject}</span>
        ) : null}
        {showCollapseToggle ? (
          <SegmentToggle collapsed={!!collapsed} onToggle={() => onToggleCollapse?.()} />
        ) : null}
        {running ? <Loader2 className="action-row__spin" size={12} /> : null}
        {expandable ? (
          <button
            type="button"
            className={`action-row__detail-toggle${detailOpen ? " action-row__detail-toggle--open" : ""}`}
            onClick={() => setDetailOpen((v) => !v)}
            aria-expanded={detailOpen}
            aria-label={detailOpen ? t("actionStream.hideDetails") : t("actionStream.showDetails")}
          >
            <ChevronRight size={11} />
          </button>
        ) : null}
      </div>
      {detailOpen && hasDetails ? (
        <div className="action-row__detail">
          {item.error ? <div className="action-row__err">{item.error}</div> : null}
          {item.isShell && item.output ? (
            <CodeViewer flat value={item.output} maxHeight={260} />
          ) : null}
          {!item.isShell && item.output ? <CodeViewer flat value={item.output} maxHeight={220} /> : null}
          {!item.output && item.args ? <CodeViewer flat value={prettyJson(item.args)} language="json" maxHeight={160} /> : null}
          {item.truncated ? <div className="action-row__note">{t("tool.truncated")}</div> : null}
        </div>
      ) : null}
    </div>
  );
});

function SegmentToggle({
  collapsed,
  onToggle,
}: {
  collapsed: boolean;
  onToggle: () => void;
}) {
  const t = useT();
  return (
    <button
      type="button"
      className={`action-row__segment-toggle${collapsed ? "" : " action-row__segment-toggle--open"}`}
      onClick={onToggle}
      aria-expanded={!collapsed}
      aria-label={collapsed ? t("actionStream.expandSegment") : t("actionStream.collapseSegment")}
    >
      <ChevronRight size={12} />
    </button>
  );
}

function ThinkingRow({
  status,
  showCollapseToggle,
  collapsed,
  onToggleCollapse,
}: {
  status: SegmentEntry & { kind: "thinking" };
  showCollapseToggle?: boolean;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
}) {
  return (
    <div className={`action-row action-row--thinking action-row--${status.status}`}>
      <div className="action-row__main">
        <span className="action-row__verb">{verbForThinking(status.status)}</span>
        {showCollapseToggle ? (
          <SegmentToggle collapsed={!!collapsed} onToggle={() => onToggleCollapse?.()} />
        ) : null}
        {status.status === "running" ? <Loader2 className="action-row__spin" size={12} /> : null}
      </div>
    </div>
  );
}

export const ActionSegmentView = memo(function ActionSegmentView({
  segment,
  workspaceRoot,
  onOpenFile,
}: {
  segment: ActionSegment;
  workspaceRoot: string;
  onOpenFile?: (req: ActionFileOpenRequest) => void;
}) {
  const [collapsed, setCollapsed] = useState(segment.complete);
  const wasComplete = useRef(segment.complete);

  useEffect(() => {
    if (segment.complete && !wasComplete.current) {
      setCollapsed(true);
    }
    wasComplete.current = segment.complete;
  }, [segment.complete]);

  if (segment.entries.length === 0) return null;

  const first = segment.entries[0]!;
  const hiddenEntries = collapsed ? segment.entries.slice(1) : segment.entries.slice(1);

  const renderEntry = (entry: SegmentEntry, index: number) => {
    const showCollapseToggle = index === 0;
    if (entry.kind === "thinking") {
      return (
        <ThinkingRow
          key={entry.id}
          status={entry}
          showCollapseToggle={showCollapseToggle}
          collapsed={collapsed}
          onToggleCollapse={() => setCollapsed((v) => !v)}
        />
      );
    }
    const rows = [
      <ActionToolRow
        key={entry.item.id}
        item={entry.item}
        workspaceRoot={workspaceRoot}
        showCollapseToggle={showCollapseToggle}
        collapsed={collapsed}
        onToggleCollapse={() => setCollapsed((v) => !v)}
        onOpenFile={onOpenFile}
      />,
    ];
    for (const sub of entry.subcalls ?? []) {
      rows.push(
        <ActionToolRow
          key={sub.id}
          item={sub}
          workspaceRoot={workspaceRoot}
          onOpenFile={onOpenFile}
        />,
      );
    }
    return rows;
  };

  return (
    <div className={`action-segment${collapsed ? " action-segment--collapsed" : ""}${segment.complete ? " action-segment--complete" : ""}`}>
      {renderEntry(first, 0)}
      {!collapsed && hiddenEntries.flatMap((entry, idx) => renderEntry(entry, idx + 1))}
    </div>
  );
});
