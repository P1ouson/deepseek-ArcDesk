import { memo, useEffect, useRef, useState } from "react";
import { MotionUnfold } from "./MotionUnfold";
import { ChevronRight, Loader2 } from "lucide-react";
import { AnchoredPopover } from "./AnchoredPopover";
import { CodeViewer } from "./CodeViewer";
import { FileTypeIcon } from "./FileTypeIcon";
import {
  actionContextForTool,
  deriveThinkingBlockHint,
  deriveThinkingBlockTitle,
  verbLabelForTool,
} from "../lib/thinkingBlockLabels";
import {
  type ActionFileRef,
  type ActionSegment,
  type SegmentEntry,
  type ThinkingBlock,
  filesForTool,
  isWriteTool,
  subjectLabel,
  thinkingBlockIsActive,
} from "../lib/actionStream";
import type { ToolItem } from "../lib/actionStream";
import type { LiveStream } from "../lib/useController";
import type { ToolFileDiff } from "../lib/tools";
import { useT } from "../lib/i18n";
import { prettyJson } from "../lib/prettyJson";

export interface ActionFileOpenRequest {
  path: string;
  diff?: ToolFileDiff;
}

function ActionFileLinks({
  files,
  onOpenFile,
}: {
  files: ActionFileRef[];
  onOpenFile?: (req: ActionFileOpenRequest) => void;
}) {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement>(null);
  const resolved = files.length > 0 ? files : [];

  if (resolved.length === 0) return null;

  const primary = resolved[0]!;
  const clickable = !!onOpenFile && resolved.length > 0;

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
  const verb = verbLabelForTool(item.name, item.status, t);
  const context = actionContextForTool(item, t);
  const files = filesForTool(item, workspaceRoot);
  const subject = context ? "" : subjectLabel(item);
  const hasFileLinks = files.length > 0;
  const hasDetails = !!(item.output || item.error || (item.args && !hasFileLinks && !context));
  const expandable = hasDetails;

  useEffect(() => {
    if (running && (item.output || item.error)) setDetailOpen(true);
  }, [running, item.output, item.error]);

  const handleFileOpen = (req: ActionFileOpenRequest) => {
    if (isWriteTool(item.name) && item.fileDiff?.diff) {
      onOpenFile?.({ path: req.path, diff: item.fileDiff });
      return;
    }
    onOpenFile?.(req);
  };

  return (
    <div
      className={`action-row action-row--${item.status}${item.readOnly ? " action-row--readonly" : ""}`}
    >
      <div className="action-row__main">
        <span className="action-row__verb">{verb}</span>
        {hasFileLinks ? (
          <ActionFileLinks files={files} onOpenFile={handleFileOpen} />
        ) : subject ? (
          <span className="action-row__subject">{subject}</span>
        ) : context ? (
          <span className="action-row__subject action-row__subject--context">{context}</span>
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
      {hasDetails ? (
        <MotionUnfold open={detailOpen}>
          <div className="action-row__detail">
            {item.error ? <div className="action-row__err">{item.error}</div> : null}
            {item.output ? <CodeViewer flat value={item.output} maxHeight={220} /> : null}
            {!item.output && item.args ? <CodeViewer flat value={prettyJson(item.args)} language="json" maxHeight={160} /> : null}
            {item.truncated ? (
              <div className="action-row__note">
                {running ? t("tool.segmentedRead") : t("tool.truncated")}
              </div>
            ) : null}
          </div>
        </MotionUnfold>
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
  const t = useT();
  const verb =
    status.status === "running" ? t("thinkingBlock.verb.thinking") : t("thinkingBlock.verbDone.thinking");
  return (
    <div className={`action-row action-row--thinking action-row--${status.status}`}>
      <div className="action-row__main">
        <span className="action-row__verb">{verb}</span>
        {showCollapseToggle ? (
          <SegmentToggle collapsed={!!collapsed} onToggle={() => onToggleCollapse?.()} />
        ) : null}
        {status.status === "running" ? <Loader2 className="action-row__spin" size={12} /> : null}
      </div>
    </div>
  );
}

function renderSegmentEntry(
  entry: SegmentEntry,
  workspaceRoot: string,
  onOpenFile?: (req: ActionFileOpenRequest) => void,
  options?: {
    showCollapseToggle?: boolean;
    collapsed?: boolean;
    onToggleCollapse?: () => void;
  },
) {
  if (entry.kind === "thinking") {
    return (
      <ThinkingRow
        key={entry.id}
        status={entry}
        showCollapseToggle={options?.showCollapseToggle}
        collapsed={options?.collapsed}
        onToggleCollapse={options?.onToggleCollapse}
      />
    );
  }
  const rows = [
    <ActionToolRow
      key={entry.item.id}
      item={entry.item}
      workspaceRoot={workspaceRoot}
      showCollapseToggle={options?.showCollapseToggle}
      collapsed={options?.collapsed}
      onToggleCollapse={options?.onToggleCollapse}
      onOpenFile={onOpenFile}
    />,
  ];
  for (const sub of entry.subcalls ?? []) {
    rows.push(
      <ActionToolRow key={sub.id} item={sub} workspaceRoot={workspaceRoot} onOpenFile={onOpenFile} />,
    );
  }
  return rows;
}

export const ThinkingBlockView = memo(function ThinkingBlockView({
  block,
  workspaceRoot,
  onOpenFile,
  live,
}: {
  block: ThinkingBlock;
  workspaceRoot: string;
  onOpenFile?: (req: ActionFileOpenRequest) => void;
  live?: LiveStream;
}) {
  const t = useT();
  const active = thinkingBlockIsActive(block);
  const title = deriveThinkingBlockTitle(block, t);
  const meta = deriveThinkingBlockHint(block, t);
  const liveAttached =
    live != null && block.streaming && live.reasoning.trim().length > 0;
  const displayReasoning =
    liveAttached && live.reasoning.trim()
      ? block.reasoning.trim() && !block.reasoning.includes(live.reasoning.trim())
        ? `${block.reasoning.trim()}\n\n${live.reasoning}`
        : live.reasoning || block.reasoning
      : block.reasoning;
  const hasReasoning = displayReasoning.trim().length > 0;
  const hasTools = block.entries.length > 0;
  const [open, setOpen] = useState(() => active || hasTools);

  // Auto-expand while the block is active; keep user-expanded state after the turn
  // finishes so tool calls and reasoning stay visible (do not auto-collapse).
  useEffect(() => {
    if (active) setOpen(true);
  }, [active]);

  if (!hasReasoning && !hasTools) return null;

  return (
    <div className={`reasoning thinking-block${active ? " thinking-block--active" : ""}`}>
      <button type="button" className="reasoning__toggle thinking-block__toggle" onClick={() => setOpen((v) => !v)}>
        <ChevronRight className={`reasoning__chevron ${open ? "reasoning__chevron--open" : ""}`} size={12} />
        <span className="thinking-block__title">{title}</span>
        {active ? <span className="thinking-block__badge">{t("thinkingBlock.badgeAuto")}</span> : null}
        {active && !hasReasoning && hasTools ? <Loader2 className="thinking-block__spin" size={12} /> : null}
      </button>
      {meta ? <div className="thinking-block__meta">{meta}</div> : null}
      <MotionUnfold open={open}>
        <div className="reasoning__body">
          {hasReasoning ? (
            <>
              {displayReasoning}
              {active && block.streaming ? <span className="cursor" /> : null}
            </>
          ) : null}
          {hasTools ? (
            <div className={`thinking-block__tools${hasReasoning ? "" : " thinking-block__tools--only"}`}>
              {block.entries.flatMap((entry) => renderSegmentEntry(entry, workspaceRoot, onOpenFile))}
            </div>
          ) : null}
        </div>
      </MotionUnfold>
    </div>
  );
});

export const ActionSegmentView = memo(function ActionSegmentView({
  segment,
  workspaceRoot,
  onOpenFile,
}: {
  segment: ActionSegment;
  workspaceRoot: string;
  onOpenFile?: (req: ActionFileOpenRequest) => void;
}) {
  const [collapsed, setCollapsed] = useState(false);

  if (segment.entries.length === 0) return null;

  const first = segment.entries[0]!;
  const hiddenEntries = collapsed ? segment.entries.slice(1) : segment.entries.slice(1);

  const renderEntry = (entry: SegmentEntry, index: number) =>
    renderSegmentEntry(entry, workspaceRoot, onOpenFile, {
      showCollapseToggle: index === 0,
      collapsed,
      onToggleCollapse: () => setCollapsed((v) => !v),
    });

  return (
    <div className={`action-segment${collapsed ? " action-segment--collapsed" : ""}${segment.complete ? " action-segment--complete" : ""}`}>
      {renderEntry(first, 0)}
      {hiddenEntries.length > 0 ? (
        <MotionUnfold open={!collapsed}>
          <div className="action-segment__more">
            {hiddenEntries.flatMap((entry, idx) => renderEntry(entry, idx + 1))}
          </div>
        </MotionUnfold>
      ) : null}
    </div>
  );
});
