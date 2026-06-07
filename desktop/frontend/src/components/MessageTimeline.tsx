import { useEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, Info } from "lucide-react";
import { VList, type VListHandle } from "virtua";
import type { Item, LiveStream } from "../lib/useController";
import { useT } from "../lib/i18n";
import { AssistantMessage, UserMessage } from "./Message";
import { ToolCard } from "./ToolCard";
import { SessionMetricsBar } from "./SessionMetricsBar";
import { Welcome } from "./Welcome";
import { CopyButton } from "./CopyButton";
import type { CheckpointMeta, BalanceInfo, WireUsage } from "../lib/types";

type ToolItem = Extract<Item, { kind: "tool" }>;

function questionAnchorId(id: string): string {
  return `question-anchor-${id}`;
}

export interface MessageTimelineProps {
  tabId?: string;
  items: Item[];
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
      {open && item.summary ? <pre className="msg-compaction__summary">{item.summary}</pre> : null}
    </div>
  );
}

function TimelineRow({
  item,
  live,
  subcallsByParent,
  userTurn,
  checkpointsByTurn,
  openTurn,
  onToggleRewind,
  onRewind,
  actionPending,
  rewindDisabled,
}: {
  item: Item;
  live?: LiveStream;
  subcallsByParent: Map<string, ToolItem[]>;
  userTurn: Map<string, number>;
  checkpointsByTurn: Map<number, CheckpointMeta>;
  openTurn: number | null;
  onToggleRewind: (turn: number | null) => void;
  onRewind?: (turn: number, scope: string) => void;
  actionPending?: boolean;
  rewindDisabled?: boolean;
}) {
  switch (item.kind) {
    case "user": {
      const turn = userTurn.get(item.id);
      return (
        <UserMessage
          text={item.text}
          turn={turn}
          anchorId={questionAnchorId(item.id)}
          open={turn != null && openTurn === turn}
          onToggle={() => onToggleRewind(turn ?? null)}
          checkpoint={turn != null ? checkpointsByTurn.get(turn) : undefined}
          actionPending={actionPending}
          rewindDisabled={rewindDisabled}
          onRewind={(tn, scope) => {
            onRewind?.(tn, scope);
            onToggleRewind(null);
          }}
        />
      );
    }
    case "assistant": {
      const shown = live && live.id === item.id ? { ...item, text: live.text, reasoning: live.reasoning, streaming: true } : item;
      return (
        <div className="msg-assistant">
          <CopyButton text={shown.text} className="msg-assistant__copy" />
          <AssistantMessage item={shown} />
        </div>
      );
    }
    case "tool":
      if (item.parentId) return null;
      if (item.name === "todo_write" || item.name === "exit_plan_mode") return null;
      return <ToolCard item={item} subcalls={subcallsByParent.get(item.id)} />;
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
    default:
      return null;
  }
}

export function MessageTimeline({
  tabId,
  items,
  live,
  usage,
  sessionCost,
  sessionCurrency,
  balance,
  footerHeight = 0,
  checkpoints = [],
  actionPending = false,
  rewindDisabled = false,
  onPrompt,
  onRewind,
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

  const checkpointsByTurn = useMemo(() => new Map(checkpoints.map((cp) => [cp.turn, cp])), [checkpoints]);

  const rows = useMemo(() => items.filter((it) => !(it.kind === "tool" && it.parentId)), [items]);
  const empty = rows.length === 0;

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
    const ro = new ResizeObserver(() => setListHeight(el.clientHeight));
    ro.observe(el);
    setListHeight(el.clientHeight);
    return () => ro.disconnect();
  }, [empty]);

  useEffect(() => {
    if (!stick.current || empty) return;
    listRef.current?.scrollToIndex(rows.length - 1, { align: "end" });
  }, [rows.length, live?.text?.length, live?.reasoning?.length, empty]);

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
    listRef.current?.scrollToIndex(rows.length - 1, { align: "end", smooth: true });
    setShowFab(false);
  };

  if (empty) {
    return (
      <div className="timeline timeline--empty">
        <div className="welcome-empty">
          <SessionMetricsBar
            tabId={tabId}
            usage={usage}
            sessionCost={sessionCost}
            sessionCurrency={sessionCurrency}
            balance={balance}
          />
          <Welcome onPrompt={onPrompt} variant="workbench" />
        </div>
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
        {rows.map((item) => (
          <div key={item.id} className="timeline__turn">
            <TimelineRow
              item={item}
              live={live}
              subcallsByParent={subcallsByParent}
              userTurn={userTurn}
              checkpointsByTurn={checkpointsByTurn}
              openTurn={openTurn}
              onToggleRewind={(turn) => setOpenTurn((cur) => (cur === turn ? null : turn))}
              onRewind={onRewind}
              actionPending={actionPending}
              rewindDisabled={rewindDisabled}
            />
          </div>
        ))}
      </VList>
      {showFab && (
        <button type="button" className="timeline__fab" onClick={scrollToBottom} aria-label={t("timeline.scrollToBottom")}>
          <ChevronDown size={18} />
        </button>
      )}
    </div>
  );
}
