import { useCallback, useEffect, useMemo, useRef, useState, memo } from "react";
import { ChevronDown, Info } from "lucide-react";
import { MotionUnfold } from "./MotionUnfold";
import { buildTimelineRows, isShellTimelineTool } from "../lib/actionStream";
import type { TimelineRow as BuiltTimelineRow } from "../lib/actionStream";
import { truncatedAssistantIds } from "../lib/responseTruncation";
import type { Item, LiveStream } from "../lib/useController";
import { useT } from "../lib/i18n";
import { ActionSegmentView, ThinkingBlockView, type ActionFileOpenRequest } from "./ActionStream";
import { AskTimelineBlock } from "./AskTimelineBlock";
import { AssistantMessage, UserMessage } from "./Message";
import { ShellCommandCard } from "./ShellCommandCard";
import { Welcome } from "./Welcome";
import type { CheckpointMeta, BalanceInfo, WireUsage } from "../lib/types";

type ToolItem = Extract<Item, { kind: "tool" }>;

function questionAnchorId(id: string): string {
  return `question-anchor-${id}`;
}

function PendingUserMessage({ text }: { text: string }) {
  const displayText = text.replace(/@\.ARCDESK\/attachments\/[^\s]+/g, "[image]");
  return (
    <div className="msg msg--user msg--pending" aria-live="polite">
      <span className="msg__caret">›</span>
      <div className="msg__text">{displayText}</div>
    </div>
  );
}

export interface MessageTimelineProps {
  tabId?: string;
  items: Item[];
  pendingUser?: string;
  live?: LiveStream;
  usage?: WireUsage;
  sessionCost?: number;
  sessionCurrency?: string;
  balance?: BalanceInfo;
  footerHeight?: number;
  checkpoints?: CheckpointMeta[];
  actionPending?: boolean;
  rewindDisabled?: boolean;
  onPrompt: (text: string) => void;
  onContinueGeneration?: (assistantId: string) => void;
  continueDisabled?: boolean;
  onRewind?: (turn: number, scope: string) => void;
  workspaceRoot?: string;
  onOpenActionFile?: (req: ActionFileOpenRequest) => void;
  showConnectionRecovery?: boolean;
  onOpenConnectionSetup?: () => void;
  /** Folder display name for empty-state headline. */
  workspaceName?: string;
  workspacePath?: string;
  showWorkspaceMesh?: boolean;
  /** Fired when the user scrolls away from / back to the bottom (for transcript priority). */
  onPinnedToBottomChange?: (pinned: boolean) => void;
}

function CompactionBlock({ item }: { item: Extract<Item, { kind: "compaction" }> }) {
  const [open, setOpen] = useState(false);
  const t = useT();
  return (
    <div className="msg-compaction">
      {t("compaction.title")} — {t("compaction.messages", { n: item.messages || 0 })}
      {item.summary ? (
        <button type="button" className="msg-compaction__toggle" onClick={() => setOpen((v) => !v)}>
          {open ? t("compaction.hideSummary") : t("compaction.showSummary")}
        </button>
      ) : null}
      {item.summary ? (
        <MotionUnfold open={open}>
          <pre className="msg-compaction__summary">{item.summary}</pre>
        </MotionUnfold>
      ) : null}
    </div>
  );
}

const TimelineRow = memo(function TimelineRow({
  item,
  live,
  userTurn,
  assistantTurn,
  checkpointsByTurn,
  openTurn,
  onToggleRewind,
  onRewind,
  actionPending,
  rewindDisabled,
  turnCopyText,
  showTurnActions,
  showContinue,
  continueDisabled,
  onContinue,
}: {
  item: Item;
  live?: LiveStream;
  userTurn: Map<string, number>;
  assistantTurn: Map<string, number>;
  checkpointsByTurn: Map<number, CheckpointMeta>;
  openTurn: number | null;
  onToggleRewind: (turn: number | null) => void;
  onRewind?: (turn: number, scope: string) => void;
  actionPending?: boolean;
  rewindDisabled?: boolean;
  turnCopyText?: string;
  showTurnActions?: boolean;
  showContinue?: boolean;
  continueDisabled?: boolean;
  onContinue?: () => void;
}) {
  switch (item.kind) {
    case "user": {
      const turn = userTurn.get(item.id);
      return (
        <UserMessage
          text={item.text}
          turn={turn}
          anchorId={questionAnchorId(item.id)}
        />
      );
    }
    case "assistant": {
      const shown = live && live.id === item.id ? { ...item, text: live.text, reasoning: live.reasoning, streaming: true } : item;
      const turn = assistantTurn.get(item.id);
      return (
        <AssistantMessage
          item={shown}
          turn={turn}
          open={turn != null && openTurn === turn}
          onToggle={() => onToggleRewind(turn ?? null)}
          checkpoint={turn != null ? checkpointsByTurn.get(turn) : undefined}
          actionPending={actionPending}
          rewindDisabled={rewindDisabled}
          copyText={turnCopyText}
          showTurnActions={showTurnActions}
          showContinue={showContinue}
          continueDisabled={continueDisabled}
          onContinue={onContinue}
          onRewind={(tn, scope) => {
            onRewind?.(tn, scope);
            onToggleRewind(null);
          }}
        />
      );
    }
    case "phase":
      return (
        <div className="msg-system">
          <Info size={12} style={{ display: "inline", verticalAlign: "middle", marginRight: 4 }} />
          {item.text}
        </div>
      );
    case "notice":
      return <div className={`notice notice--${item.level}`}>{item.text}</div>;
    case "compaction":
      return <CompactionBlock item={item} />;
    case "ask":
      return (
        <AskTimelineBlock
          ask={item.ask}
          answers={item.answers}
          dismissed={item.dismissed}
          pending={item.pending}
        />
      );
    case "tool":
      if (isShellTimelineTool(item)) {
        return <ShellCommandCard item={item} />;
      }
      return null;
    default:
      return null;
  }
});

export function MessageTimeline({
  tabId,
  items,
  pendingUser,
  live,
  usage: _usage,
  sessionCost: _sessionCost,
  sessionCurrency: _sessionCurrency,
  balance: _balance,
  footerHeight = 0,
  checkpoints = [],
  actionPending = false,
  rewindDisabled = false,
  onPrompt,
  onContinueGeneration,
  continueDisabled = false,
  onRewind,
  workspaceRoot = "",
  onOpenActionFile,
  showConnectionRecovery = false,
  onOpenConnectionSetup,
  workspaceName,
  workspacePath,
  showWorkspaceMesh = false,
  onPinnedToBottomChange,
}: MessageTimelineProps) {
  const t = useT();
  const containerRef = useRef<HTMLDivElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const stick = useRef(true);
  const prevItemCountRef = useRef(0);
  const [pinnedToBottom, setPinnedToBottom] = useState(true);
  const [listHeight, setListHeight] = useState(480);
  const [showFab, setShowFab] = useState(false);
  const [openTurn, setOpenTurn] = useState<number | null>(null);

  const subcallsByParent = useMemo(() => {
    const m = new Map<string, ToolItem[]>();
    for (const it of items) {
      if (it.kind === "tool" && it.parentId) {
        const arr = m.get(it.parentId) ?? [];
        arr.push(it);
        m.set(it.parentId, arr);
      }
    }
    return m;
  }, [items]);

  const userTurn = useMemo(() => {
    const map = new Map<string, number>();
    let turn = 0;
    for (const it of items) {
      if (it.kind !== "user") continue;
      map.set(it.id, turn);
      turn += 1;
    }
    return map;
  }, [items]);

  const assistantTurn = useMemo(() => {
    const map = new Map<string, number>();
    let lastTurn: number | undefined;
    for (const it of items) {
      if (it.kind === "user") {
        lastTurn = userTurn.get(it.id);
      } else if (it.kind === "assistant" && lastTurn != null) {
        map.set(it.id, lastTurn);
      }
    }
    return map;
  }, [items, userTurn]);

  const checkpointsByTurn = useMemo(() => new Map(checkpoints.map((cp) => [cp.turn, cp])), [checkpoints]);

  const truncatedAssistants = useMemo(() => truncatedAssistantIds(items), [items]);

  const turnAssistantActions = useMemo(() => {
    const copyTextByAssistantId = new Map<string, string>();
    const showActionsByAssistantId = new Map<string, boolean>();
    let turn: number | undefined;
    const chunks: string[] = [];
    let lastTextAssistantId: string | undefined;

    const flush = () => {
      if (lastTextAssistantId && chunks.length > 0) {
        copyTextByAssistantId.set(lastTextAssistantId, chunks.join("\n\n"));
        showActionsByAssistantId.set(lastTextAssistantId, true);
      }
      chunks.length = 0;
      lastTextAssistantId = undefined;
    };

    for (const it of items) {
      if (it.kind === "user") {
        flush();
        turn = userTurn.get(it.id);
      } else if (it.kind === "assistant" && turn != null) {
        const text = it.text.trim();
        if (text) {
          chunks.push(text);
          lastTextAssistantId = it.id;
        }
      }
    }
    flush();
    return { copyTextByAssistantId, showActionsByAssistantId };
  }, [items, userTurn]);

  const rows = useMemo(
    () => buildTimelineRows(items, subcallsByParent, pinnedToBottom ? live : undefined),
    [items, subcallsByParent, live, pinnedToBottom],
  );
  const empty = items.length === 0 && !pendingUser;

  const rowKey = (row: BuiltTimelineRow) => {
    if (row.kind === "thinking-block") return row.block.id;
    if (row.kind === "action-segment") return row.segment.id;
    return row.item.id;
  };
  const pendingRowCount = pendingUser ? 1 : 0;
  const totalRows = rows.length + pendingRowCount;

  const pinToBottom = useCallback(() => {
    stick.current = true;
    setPinnedToBottom((prev) => {
      if (!prev) onPinnedToBottomChange?.(true);
      return true;
    });
    setShowFab(false);
  }, [onPinnedToBottomChange]);

  const scrollTimelineToBottom = useCallback((behavior: ScrollBehavior = "auto") => {
    const el = scrollRef.current;
    if (!el) return;
    if (behavior === "smooth") {
      el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
    } else {
      el.scrollTop = el.scrollHeight;
    }
  }, []);

  useEffect(() => {
    pinToBottom();
  }, [tabId, pinToBottom]);

  useEffect(() => {
    const hydrated = prevItemCountRef.current === 0 && items.length > 0;
    prevItemCountRef.current = items.length;
    if (hydrated) pinToBottom();
  }, [items.length, pinToBottom]);

  useEffect(() => {
    if (openTurn == null) return;
    const onDown = (e: MouseEvent) => {
      const el = e.target as HTMLElement | null;
      if (!el?.closest(".rewind")) setOpenTurn(null);
    };
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [openTurn]);

  useEffect(() => {
    const el = containerRef.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    let frame = 0;
    const ro = new ResizeObserver(() => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => setListHeight(el.clientHeight));
    });
    ro.observe(el);
    setListHeight(el.clientHeight);
    return () => {
      cancelAnimationFrame(frame);
      ro.disconnect();
    };
  }, [empty, pendingUser]);

  useEffect(() => {
    if (!stick.current || (empty && !pendingUser)) return;
    let outer = 0;
    let inner = 0;
    outer = requestAnimationFrame(() => {
      inner = requestAnimationFrame(() => {
        scrollTimelineToBottom("auto");
      });
    });
    return () => {
      cancelAnimationFrame(outer);
      cancelAnimationFrame(inner);
    };
  }, [
    totalRows,
    rows.length,
    pinnedToBottom ? live?.text?.length : 0,
    pinnedToBottom ? live?.reasoning?.length : 0,
    empty,
    pendingUser,
    pinnedToBottom,
    tabId,
    scrollTimelineToBottom,
  ]);

  const renderRow = (row: BuiltTimelineRow) => (
    <div key={rowKey(row)} className="timeline__turn">
      {row.kind === "thinking-block" ? (
        <ThinkingBlockView
          block={row.block}
          workspaceRoot={workspaceRoot}
          onOpenFile={onOpenActionFile}
          live={pinnedToBottom ? live : undefined}
        />
      ) : row.kind === "action-segment" ? (
        <ActionSegmentView
          segment={row.segment}
          workspaceRoot={workspaceRoot}
          onOpenFile={onOpenActionFile}
        />
      ) : (
        <TimelineRow
          item={row.item}
          live={pinnedToBottom ? live : undefined}
          userTurn={userTurn}
          assistantTurn={assistantTurn}
          checkpointsByTurn={checkpointsByTurn}
          openTurn={openTurn}
          onToggleRewind={(turn) => setOpenTurn((cur) => (cur === turn ? null : turn))}
          onRewind={onRewind}
          actionPending={actionPending}
          rewindDisabled={rewindDisabled}
          turnCopyText={turnAssistantActions.copyTextByAssistantId.get(row.item.id)}
          showTurnActions={turnAssistantActions.showActionsByAssistantId.get(row.item.id) === true}
          showContinue={
            row.item.kind === "assistant" &&
            turnAssistantActions.showActionsByAssistantId.get(row.item.id) === true &&
            truncatedAssistants.has(row.item.id) &&
            live?.id !== row.item.id
          }
          continueDisabled={continueDisabled}
          onContinue={
            row.item.kind === "assistant" && onContinueGeneration
              ? () => onContinueGeneration(row.item.id)
              : undefined
          }
        />
      )}
    </div>
  );

  const onScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const max = Math.max(0, el.scrollHeight - el.clientHeight);
    const atBottom = max - el.scrollTop < 80;
    stick.current = atBottom;
    setPinnedToBottom((prev) => {
      if (prev !== atBottom) onPinnedToBottomChange?.(atBottom);
      return prev === atBottom ? prev : atBottom;
    });
    setShowFab(!atBottom);
  };

  const scrollToBottom = () => {
    pinToBottom();
    scrollTimelineToBottom("smooth");
  };

  if (empty) {
    return (
      <div className="timeline timeline--empty">
        <Welcome
          onPrompt={onPrompt}
          variant="code"
          disabled={actionPending || rewindDisabled}
          showConnectionRecovery={showConnectionRecovery}
          onOpenConnectionSetup={onOpenConnectionSetup}
          workspaceName={workspaceName}
          workspacePath={workspacePath}
          showWorkspaceMesh={showWorkspaceMesh}
        />
      </div>
    );
  }

  if (items.length === 0 && pendingUser) {
    return (
      <div className="timeline timeline--pending-only" ref={containerRef} style={{ flex: 1, minHeight: 0, padding: "16px 20px", paddingBottom: footerHeight + 8 }}>
        <PendingUserMessage text={pendingUser} />
      </div>
    );
  }

  return (
    <div className="timeline" ref={containerRef} style={{ flex: 1, minHeight: 0 }}>
      <div
        ref={scrollRef}
        className="timeline__static"
        style={{ height: listHeight, overflow: "auto", padding: "16px 20px", paddingBottom: footerHeight + 8 }}
        onScroll={onScroll}
      >
        {rows.map((row) => renderRow(row))}
        {pendingUser ? (
          <div key="pending-user" className="timeline__turn">
            <PendingUserMessage text={pendingUser} />
          </div>
        ) : null}
      </div>
      {showFab && (
        <button type="button" className="timeline__fab" onClick={scrollToBottom} aria-label={t("timeline.scrollToBottom")}>
          <ChevronDown size={18} />
        </button>
      )}
    </div>
  );
}
