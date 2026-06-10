import { useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, Info } from "lucide-react";
import { MotionUnfold } from "./MotionUnfold";
import { VList, type VListHandle } from "virtua";
import { buildTimelineRows } from "../lib/actionStream";
import type { TimelineRow as BuiltTimelineRow } from "../lib/actionStream";
import type { Item, LiveStream } from "../lib/useController";
import { useT } from "../lib/i18n";
import { ActionSegmentView, type ActionFileOpenRequest } from "./ActionStream";
import { AskTimelineBlock } from "./AskTimelineBlock";
import { AssistantMessage, UserMessage } from "./Message";
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
  onRewind?: (turn: number, scope: string) => void;
  workspaceRoot?: string;
  onOpenActionFile?: (req: ActionFileOpenRequest) => void;
  showConnectionRecovery?: boolean;
  onOpenConnectionSetup?: () => void;
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

function TimelineRow({
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
  showTurnCopy,
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
  showTurnCopy?: boolean;
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
          showCopyButton={showTurnCopy}
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
    default:
      return null;
  }
}

export function MessageTimeline({
  tabId: _tabId,
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
  onRewind,
  workspaceRoot = "",
  onOpenActionFile,
  showConnectionRecovery = false,
  onOpenConnectionSetup,
}: MessageTimelineProps) {
  const t = useT();
  const containerRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<VListHandle>(null);
  const stick = useRef(true);
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

  const turnAssistantCopy = useMemo(() => {
    const copyTextByAssistantId = new Map<string, string>();
    const showCopyByAssistantId = new Map<string, boolean>();
    let turn: number | undefined;
    const chunks: string[] = [];
    let lastAssistantId: string | undefined;

    const flush = () => {
      if (lastAssistantId && chunks.length > 0) {
        copyTextByAssistantId.set(lastAssistantId, chunks.join("\n\n"));
        showCopyByAssistantId.set(lastAssistantId, true);
      }
      chunks.length = 0;
      lastAssistantId = undefined;
    };

    for (const it of items) {
      if (it.kind === "user") {
        flush();
        turn = userTurn.get(it.id);
      } else if (it.kind === "assistant" && turn != null) {
        lastAssistantId = it.id;
        const text = it.text.trim();
        if (text) chunks.push(text);
      }
    }
    flush();
    return { copyTextByAssistantId, showCopyByAssistantId };
  }, [items, userTurn]);

  const rows = useMemo(
    () => buildTimelineRows(items, subcallsByParent, live),
    [items, subcallsByParent, live],
  );
  const empty = items.length === 0 && !pendingUser;

  const rowKey = (row: BuiltTimelineRow) => (row.kind === "action-segment" ? row.segment.id : row.item.id);
  const pendingRowCount = pendingUser ? 1 : 0;
  const totalRows = rows.length + pendingRowCount;

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
    listRef.current?.scrollToIndex(totalRows - 1, { align: "end" });
  }, [totalRows, rows.length, live?.text?.length, live?.reasoning?.length, empty, pendingUser]);

  const onScroll = (offset: number) => {
    const el = containerRef.current;
    if (!el) return;
    const max = Math.max(0, el.scrollHeight - listHeight);
    const atBottom = max - offset < 80;
    stick.current = atBottom;
    setShowFab(!atBottom);
  };

  const scrollToBottom = () => {
    stick.current = true;
    listRef.current?.scrollToIndex(totalRows - 1, { align: "end", smooth: true });
    setShowFab(false);
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
      <VList
        ref={listRef}
        style={{ height: listHeight, padding: "16px 20px", paddingBottom: footerHeight + 8 }}
        onScroll={onScroll}
      >
        {rows.map((row) => (
          <div key={rowKey(row)} className="timeline__turn">
            {row.kind === "action-segment" ? (
              <ActionSegmentView
                segment={row.segment}
                workspaceRoot={workspaceRoot}
                onOpenFile={onOpenActionFile}
              />
            ) : (
              <TimelineRow
                item={row.item}
                live={live}
                userTurn={userTurn}
                assistantTurn={assistantTurn}
                checkpointsByTurn={checkpointsByTurn}
                openTurn={openTurn}
                onToggleRewind={(turn) => setOpenTurn((cur) => (cur === turn ? null : turn))}
                onRewind={onRewind}
                actionPending={actionPending}
                rewindDisabled={rewindDisabled}
                turnCopyText={turnAssistantCopy.copyTextByAssistantId.get(row.item.id)}
                showTurnCopy={turnAssistantCopy.showCopyByAssistantId.get(row.item.id) === true}
              />
            )}
          </div>
        ))}
        {pendingUser ? (
          <div key="pending-user" className="timeline__turn">
            <PendingUserMessage text={pendingUser} />
          </div>
        ) : null}
      </VList>
      {showFab && (
        <button type="button" className="timeline__fab" onClick={scrollToBottom} aria-label={t("timeline.scrollToBottom")}>
          <ChevronDown size={18} />
        </button>
      )}
    </div>
  );
}
