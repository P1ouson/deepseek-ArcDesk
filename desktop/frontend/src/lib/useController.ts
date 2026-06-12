// useController is the frontend's state machine over the agent's event stream. It
// maintains per-tab state so background tabs preserve their streaming output, tool
// states, and approvals when the user switches away and back. The active tab's state
// is what components render.

import { useCallback, useEffect, useRef, useState } from "react";
import { shouldBlockAgentSend } from "./agentActivity";
import { asArray } from "./array";
import { app, onEvent, onReady, onTabsShell } from "./bridge";
import { sameWorkspaceRoot } from "./composerWorkspace";
import { toErrorMessage } from "./errors";
import { logBridgeError } from "./logBridgeError";
import {
  BOOT_READY_MAX_POLLS,
  BOOT_READY_POLL_MS,
  IPC_BALANCE_TIMEOUT_MS,
  IPC_HYDRATE_TIMEOUT_MS,
  IPC_META_TIMEOUT_MS,
  withIPCTimeout,
} from "./ipc";
import { notifyBackgroundTabDecision } from "./agentNotifications";
import { notifyTabMetasChanged } from "./events";
import { t } from "./i18n";
import { recordCompactionActivity, recordUsageActivity } from "./usageActivity";
import type {
  BalanceInfo,
  CheckpointMeta,
  ContextInfo,
  EffortInfo,
  HistoryMessage,
  JobView,
  MemoryView,
  Meta,
  QuestionAnswer,
  SessionMeta,
  TabMeta,
  WireApproval,
  WireAsk,
  WireEvent,
  WireUsage,
} from "./types";

export type ToolStatus = "running" | "done" | "error" | "stopped";

export type LiveStream = { id: string; text: string; reasoning: string };
export type MessageActionScope = "fork" | "summ-from" | "summ-upto" | "conversation" | "code" | "both";
export type MessageActionState = { turn: number; scope: MessageActionScope };

export type Item =
  | { kind: "user"; id: string; text: string }
  | { kind: "assistant"; id: string; text: string; reasoning: string; streaming: boolean }
  | { kind: "phase"; id: string; text: string }
  | { kind: "notice"; id: string; level: "info" | "warn"; text: string }
  | {
      kind: "compaction";
      id: string;
      pending: boolean;
      trigger: string;
      messages: number;
      summary: string;
      archive: string;
    }
  | {
      kind: "tool";
      id: string;
      name: string;
      args: string;
      readOnly: boolean;
      status: ToolStatus;
      output?: string;
      error?: string;
      truncated?: boolean;
      isShell?: boolean; // true for !-prefix shell commands (controls default expand)
      parentId?: string; // a sub-agent call nests under the `task` call with this id
      fileDiff?: { diff: string; added: number; removed: number };
    }
  | {
      kind: "ask";
      id: string;
      ask: WireAsk;
      pending: boolean;
      dismissed?: boolean;
      answers?: QuestionAnswer[];
    };

export interface State {
  items: Item[];
  running: boolean;
  turnActive: boolean;
  approval?: WireApproval;
  ask?: WireAsk;
  usage?: WireUsage;
  context: ContextInfo;
  meta?: Meta;
  balance?: BalanceInfo;
  effort?: EffortInfo;
  jobs: JobView[];
  checkpoints: CheckpointMeta[];
  messageAction?: MessageActionState;
  currentAssistant?: string;
  live?: LiveStream;
  pendingUser?: string;
  discardTurn?: boolean;
  turnStartAt: number;
  turnTokens: number;
  sessionCost: number;
  sessionCurrency: string;
  retry?: { attempt: number; max: number };
  recentlyCompleted: boolean;
  seq: number;
}

const initialState: State = {
  items: [],
  running: false,
  turnActive: false,
  context: { used: 0, window: 0 },
  jobs: [],
  checkpoints: [],
  turnStartAt: 0,
  turnTokens: 0,
  sessionCost: 0,
  sessionCurrency: "¥",
  recentlyCompleted: false,
  seq: 0,
};

export type Action =
  | { type: "event"; e: WireEvent }
  | { type: "user"; text: string }
  | { type: "unsend" }
  | { type: "meta"; meta: Meta }
  | { type: "context"; context: ContextInfo }
  | { type: "balance"; balance: BalanceInfo }
  | { type: "effort"; effort: EffortInfo }
  | { type: "jobs"; jobs: JobView[] }
  | { type: "checkpoints"; checkpoints: CheckpointMeta[] }
  | { type: "message_action_start"; action: MessageActionState }
  | { type: "message_action_done" }
  | { type: "history"; messages: HistoryMessage[] }
  | {
      type: "hydrate";
      meta: Meta;
      context: ContextInfo;
      effort: EffortInfo;
      balance?: BalanceInfo;
      jobs: JobView[];
      checkpoints: CheckpointMeta[];
      messages: HistoryMessage[];
    }
  | { type: "stream_delta"; text?: string; reasoning?: string }
  | { type: "local_notice"; level: "info" | "warn"; text: string }
  | { type: "clearApproval" }
  | { type: "clearAsk" }
  | { type: "clearRecentCompletion" }
  | { type: "resolveAsk"; id: string; answers: QuestionAnswer[]; dismissed: boolean }
  | { type: "reset" };

// ---- reducer helpers (unchanged logic) ----

function ensureAssistant(s: State): { items: Item[]; id: string; seq: number } {
  if (s.currentAssistant) {
    const exists = s.items.some((it) => it.id === s.currentAssistant && it.kind === "assistant");
    if (exists) return { items: s.items, id: s.currentAssistant, seq: s.seq };
  }
  const id = `a${s.seq}`;
  const item: Item = { kind: "assistant", id, text: "", reasoning: "", streaming: true };
  return { items: [...s.items, item], id, seq: s.seq + 1 };
}

function mergeReasoningOnlyIntoPrevious(items: Item[], assistantId: string): Item[] {
  const idx = items.findIndex((it) => it.kind === "assistant" && it.id === assistantId);
  if (idx < 0) return items;

  const current = items[idx];
  if (current.kind !== "assistant") return items;
  if (current.text.trim() || !current.reasoning.trim()) return items;

  let prevIdx = -1;
  for (let i = idx - 1; i >= 0; i--) {
    const it = items[i]!;
    if (it.kind === "user") break;
    if (it.kind === "assistant") {
      if (it.text.trim()) break;
      if (it.reasoning.trim()) {
        prevIdx = i;
        break;
      }
    }
  }
  if (prevIdx < 0) return items;

  const prev = items[prevIdx] as Extract<Item, { kind: "assistant" }>;
  const merged: Item = {
    ...prev,
    reasoning: `${prev.reasoning.trim()}\n\n${current.reasoning.trim()}`,
    streaming: false,
  };
  const next = [...items];
  next[prevIdx] = merged;
  next.splice(idx, 1);
  return next;
}

/** Commit in-flight assistant text before tool rows so pre-tool narration stays visible. */
function finalizeLiveAssistant(s: State): State {
  if (!s.live || !s.currentAssistant) return s;
  const { text, reasoning } = s.live;
  if (!text.trim() && !reasoning.trim()) return s;
  const id = s.currentAssistant;
  let items = s.items.map((it) =>
    it.kind === "assistant" && it.id === id
      ? { ...it, text, reasoning, streaming: false }
      : it,
  );
  if (!text.trim() && reasoning.trim()) {
    items = mergeReasoningOnlyIntoPrevious(items, id);
  }
  return { ...s, items, live: undefined, currentAssistant: undefined };
}

function shouldBlockConcurrentSend(state: State): boolean {
  return shouldBlockAgentSend(state);
}

const TURN_WATCHDOG_MS = 180_000;
const TURN_WATCHDOG_POLL_MS = 30_000;
const TURN_WATCHDOG_FORCE_CLEAR_MS = TURN_WATCHDOG_MS + 30_000;

function shouldArmTurnWatchdog(state: Pick<State, "running" | "turnActive" | "retry">): boolean {
  return state.running && state.turnActive && state.retry === undefined;
}

function shouldEmitTurnWatchdogNotice(
  state: Pick<State, "running" | "turnActive" | "retry" | "turnStartAt">,
  now: number,
  alreadyFiredForTurnStartAt: number | undefined,
): boolean {
  if (!shouldArmTurnWatchdog(state)) return false;
  if (alreadyFiredForTurnStartAt === state.turnStartAt) return false;
  return now - state.turnStartAt >= TURN_WATCHDOG_MS;
}

function shouldForceClearTurnWatchdog(
  state: Pick<State, "running" | "turnStartAt">,
  now: number,
  alreadyForceClearedForTurnStartAt: number | undefined,
): boolean {
  if (!state.running || !state.turnStartAt) return false;
  if (alreadyForceClearedForTurnStartAt === state.turnStartAt) return false;
  return now - state.turnStartAt >= TURN_WATCHDOG_FORCE_CLEAR_MS;
}

/** Suppress a late backend stream error after the UI already force-stopped the turn. */
function isStaleStreamDoneErr(state: Pick<State, "running" | "turnActive">, err?: string): boolean {
  if (!err || state.running || state.turnActive) return false;
  return /read stream: unexpected EOF|read stream:.*EOF/i.test(err);
}

function historyItemsFromMessages(messages: HistoryMessage[], seq: number): { items: Item[]; nextSeq: number } {
  const visible = messages.filter(
    (m) => (m.role === "user" && m.content.trim() !== "") ||
           (m.role === "assistant" && (m.content.trim() !== "" || (m.reasoning ?? "").trim() !== "")),
  );
  const items: Item[] = visible.map((m, i) =>
    m.role === "user"
      ? { kind: "user", id: `h${i}`, text: m.content }
      : { kind: "assistant", id: `h${i}`, text: m.content, reasoning: m.reasoning ?? "", streaming: false },
  );
  return { items, nextSeq: seq + visible.length };
}

function applyStreamDelta(s: State, textDelta?: string, reasoningDelta?: string): State {
  if (!textDelta && !reasoningDelta) return s;
  let base = s;
  if (base.pendingUser !== undefined) {
    base = flushPendingUser(base);
  }
  const { items, id, seq } = ensureAssistant(base);
  const liveBase = base.live?.id === id ? base.live : { id, text: "", reasoning: "" };
  const live = {
    id,
    text: liveBase.text + (textDelta ?? ""),
    reasoning: liveBase.reasoning + (reasoningDelta ?? ""),
  };
  return { ...base, items, live, currentAssistant: id, seq };
}

function isAgentBusyNotice(e: WireEvent): boolean {
  if (e.kind !== "notice") return false;
  if (e.code === "agent_busy") return true;
  const text = e.text ?? "";
  // Compatibility window: legacy wire without code (remove after one release cycle).
  return text.includes("still working") || text.includes("仍在处理");
}

function revertRejectedSend(s: State): State {
  return {
    ...s,
    running: s.turnActive,
    pendingUser: undefined,
    discardTurn: false,
  };
}

function flushPendingUser(s: State): State {
  if (s.pendingUser === undefined) return s;
  return {
    ...s,
    seq: s.seq + 1,
    items: [...s.items, { kind: "user", id: `u${s.seq}`, text: s.pendingUser }],
    pendingUser: undefined,
  };
}

function applyEvent(s: State, e: WireEvent): State {
  if (s.discardTurn) {
    if (e.kind === "turn_done") return { ...s, discardTurn: false, running: false, turnActive: false, currentAssistant: undefined, live: undefined };
    return s;
  }
  if (s.pendingUser !== undefined && e.kind !== "turn_started" && e.kind !== "turn_done") {
    if (e.kind !== "notice" || !isAgentBusyNotice(e)) {
      s = flushPendingUser(s);
    }
  }
  if (e.kind === "retrying") {
    return { ...s, retry: { attempt: e.retryAttempt ?? 0, max: e.retryMax ?? 0 } };
  }
  if (s.retry) s = { ...s, retry: undefined };
  switch (e.kind) {
    case "turn_started":
      return { ...s, running: true, turnActive: true, recentlyCompleted: false, currentAssistant: undefined, turnStartAt: Date.now(), turnTokens: 0 };
    case "text":
    case "reasoning": {
      const { items, id, seq } = ensureAssistant(s);
      const delta = e.text ?? e.reasoning ?? "";
      const base = s.live?.id === id ? s.live : { id, text: "", reasoning: "" };
      const live = e.kind === "text" ? { ...base, text: base.text + delta } : { ...base, reasoning: base.reasoning + delta };
      return { ...s, items, live, currentAssistant: id, seq };
    }
    case "message": {
      const { items, id, seq } = ensureAssistant(s);
      const next = items.map((it) =>
        it.kind === "assistant" && it.id === id
          ? { ...it, text: e.text ?? s.live?.text ?? it.text, reasoning: e.reasoning ?? s.live?.reasoning ?? it.reasoning, streaming: false }
          : it,
      );
      return { ...s, items: next, live: undefined, currentAssistant: undefined, seq };
    }
    case "tool_dispatch": {
      const t = e.tool;
      if (!t) return s;
      const id = t.id || `tool${s.seq}`;
      const idx = s.items.findIndex((it) => it.kind === "tool" && it.id === id);
      if (idx >= 0) {
        const next = [...s.items];
        const it = next[idx];
        if (it.kind === "tool") {
          next[idx] = {
            ...it,
            name: t.name,
            args: t.args ? t.args : it.args,
            readOnly: t.readOnly,
            fileDiff: t.fileDiff ?? it.fileDiff,
          };
        }
        return { ...s, items: next };
      }
      const base = finalizeLiveAssistant(s);
      return {
        ...base,
        seq: base.seq + 1,
        items: [
          ...base.items,
          {
            kind: "tool",
            id,
            name: t.name,
            args: t.args ?? "",
            readOnly: t.readOnly,
            status: "running",
            isShell: id.startsWith("shell-"),
            parentId: t.parentId,
            fileDiff: t.fileDiff,
          },
        ],
      };
    }
    case "tool_result": {
      const t = e.tool;
      if (!t) return s;
      const next = [...s.items];
      let idx = t.id ? next.findIndex((it) => it.kind === "tool" && it.id === t.id) : -1;
      if (idx < 0) {
        for (let i = next.length - 1; i >= 0; i--) {
          const it = next[i];
          if (it.kind === "tool" && it.status === "running") { idx = i; break; }
        }
      }
      if (idx >= 0) {
        const it = next[idx];
        if (it.kind === "tool") next[idx] = { ...it, status: t.err ? "error" : "done", output: t.output, error: t.err, truncated: t.truncated };
      }
      return { ...s, items: next };
    }
    case "tool_progress": {
      const t = e.tool;
      if (!t?.id) return s;
      const idx = s.items.findIndex((it) => it.kind === "tool" && it.id === t.id);
      if (idx < 0) return s;
      const next = [...s.items];
      const it = next[idx];
      if (it.kind === "tool") next[idx] = { ...it, output: (it.output ?? "") + (t.output ?? "") };
      return { ...s, items: next };
    }
    case "usage": {
      const used = e.usage && s.context.window ? e.usage.promptTokens : s.context.used;
      const turnTokens = s.turnTokens + (e.usage?.completionTokens ?? 0);
      const usageCost = e.usage?.cost ?? e.usage?.costUsd ?? 0;
      const sessionCost = s.sessionCost + usageCost;
      const sessionCurrency = e.usage?.currency || s.sessionCurrency || "¥";
      return { ...s, usage: e.usage, context: { ...s.context, used }, turnTokens, sessionCost, sessionCurrency };
    }
    case "notice": {
      const text = e.text ?? "";
      const rejected = isAgentBusyNotice(e);
      const base = rejected ? revertRejectedSend(s) : s;
      return { ...base, running: base.turnActive ? base.running : false, seq: base.seq + 1, items: [...base.items, { kind: "notice", id: `n${base.seq}`, level: e.level ?? "info", text }] };
    }
    case "phase":
      return { ...s, seq: s.seq + 1, items: [...s.items, { kind: "phase", id: `p${s.seq}`, text: e.text ?? "" }] };
    case "compaction_started":
      return { ...s, seq: s.seq + 1, items: [...s.items, { kind: "compaction", id: `c${s.seq}`, pending: true, trigger: e.compaction?.trigger ?? "", messages: 0, summary: "", archive: "" }] };
    case "compaction_done": {
      const c = e.compaction;
      const idx = [...s.items].reverse().findIndex((it) => it.kind === "compaction" && it.pending);
      const at = idx < 0 ? -1 : s.items.length - 1 - idx;
      if (!c?.summary) {
        const items = at < 0 ? s.items : s.items.filter((_, i) => i !== at);
        return { ...s, running: s.turnActive ? s.running : false, items };
      }
      const filled: Item = { kind: "compaction", id: at < 0 ? `c${s.seq}` : (s.items[at] as Extract<Item, { kind: "compaction" }>).id, pending: false, trigger: c.trigger ?? "", messages: c.messages ?? 0, summary: c.summary, archive: c.archive ?? "" };
      const items = at < 0 ? [...s.items, filled] : s.items.map((it, i) => (i === at ? filled : it));
      return { ...s, running: s.turnActive ? s.running : false, seq: s.seq + 1, items };
    }
    case "approval_request": return { ...s, approval: e.approval };
    case "ask_request": {
      const ask = e.ask;
      if (!ask) return { ...s, ask };
      const exists = s.items.some((it) => it.kind === "ask" && it.id === ask.id);
      const items = exists
        ? s.items
        : [...s.items, { kind: "ask" as const, id: ask.id, ask, pending: true }];
      return { ...s, ask, items, seq: exists ? s.seq : s.seq + 1 };
    }
    case "turn_done": {
      if (s.pendingUser !== undefined) s = flushPendingUser(s);
      const finalized = s.items.map((it) => {
        if (it.kind === "assistant" && s.live && it.id === s.live.id) return { ...it, text: s.live.text, reasoning: s.live.reasoning, streaming: false };
        if (it.kind === "assistant" && it.streaming) return { ...it, streaming: false };
        if (it.kind === "tool" && it.status === "running") return { ...it, status: "stopped" as const };
        return it;
      });
      const errText = e.err && !isStaleStreamDoneErr(s, e.err) ? e.err : undefined;
      const items: Item[] = errText
        ? [...finalized, { kind: "notice", id: `e${s.seq}`, level: "warn", text: errText }]
        : finalized;
      return {
        ...s,
        items,
        live: undefined,
        running: false,
        turnActive: false,
        recentlyCompleted: !errText,
        currentAssistant: undefined,
        approval: undefined,
        ask: undefined,
        seq: s.seq + 1,
      };
    }
    default: return s;
  }
}

function reducer(s: State, a: Action): State {
  switch (a.type) {
    case "user": return { ...s, running: true, turnStartAt: Date.now(), turnTokens: 0, pendingUser: a.text, discardTurn: false };
    case "unsend": return { ...s, pendingUser: undefined, discardTurn: true, running: false, live: undefined };
    case "meta": return { ...s, meta: a.meta };
    case "context": return { ...s, context: a.context };
    case "balance": return { ...s, balance: a.balance };
    case "effort": return { ...s, effort: a.effort };
    case "jobs": return { ...s, jobs: a.jobs };
    case "checkpoints": return { ...s, checkpoints: a.checkpoints };
    case "message_action_start": return { ...s, messageAction: a.action };
    case "message_action_done": return { ...s, messageAction: undefined };
    case "history": {
      const { items, nextSeq } = historyItemsFromMessages(a.messages, s.seq);
      return { ...s, items, seq: nextSeq };
    }
    case "hydrate": {
      const { items, nextSeq } = historyItemsFromMessages(a.messages, s.seq);
      return {
        ...s,
        meta: a.meta,
        context: a.context,
        effort: a.effort,
        balance: a.balance,
        jobs: a.jobs,
        checkpoints: a.checkpoints,
        items,
        seq: nextSeq,
      };
    }
    case "stream_delta":
      return applyStreamDelta(s, a.text, a.reasoning);
    case "local_notice": return { ...s, seq: s.seq + 1, items: [...s.items, { kind: "notice", id: `n${s.seq}`, level: a.level, text: a.text }] };
    case "clearApproval": return { ...s, approval: undefined };
    case "clearAsk": return { ...s, ask: undefined };
    case "clearRecentCompletion": return { ...s, recentlyCompleted: false };
    case "resolveAsk": {
      const items = s.items.map((it) =>
        it.kind === "ask" && it.id === a.id
          ? { ...it, answers: a.answers, dismissed: a.dismissed, pending: false }
          : it,
      );
      return { ...s, ask: undefined, items };
    }
    case "reset": return { ...initialState, meta: s.meta, context: { ...s.context, used: 0 }, balance: s.balance, effort: s.effort, jobs: s.jobs };
    case "event": return applyEvent(s, a.e);
    default: return s;
  }
}

// ---- per-tab state map ----

type TabStates = Map<string, State>;

function getOrCreateState(states: TabStates, tabId: string): State {
  if (!states.has(tabId)) states.set(tabId, { ...initialState });
  return states.get(tabId)!;
}

function messageActionBusyText(scope: MessageActionScope): string {
  switch (scope) {
    case "fork":
      return t("rewind.busyFork");
    case "summ-from":
      return t("rewind.busySummFrom");
    case "summ-upto":
      return t("rewind.busySummUpto");
    case "conversation":
      return t("rewind.busyConversation");
    case "code":
      return t("rewind.busyCode");
    default:
      return t("rewind.busyBoth");
  }
}

export function useController() {
  const statesRef = useRef<TabStates>(new Map());
  const tabTitlesRef = useRef<Map<string, string>>(new Map());
  const turnWatchdogFiredRef = useRef<Map<string, number>>(new Map());
  const turnWatchdogForceClearedRef = useRef<Map<string, number>>(new Map());
  const [activeTabId, setActiveTabId] = useState<string | undefined>();
  // A render-triggering counter so that mutations to a non-active tab's state still
  // cause a re-render when that tab becomes active.
  const [, setVersion] = useState(0);
  const [bootPhase, setBootPhase] = useState<string | null>(null);
  const [idleReady, setIdleReady] = useState(false);
  const bump = useCallback(() => setVersion((v) => v + 1), []);

  const idleMeta: Meta = {
    label: "",
    ready: true,
    eventChannel: "agent:event",
    cwd: "",
    bypass: false,
  };

  const getAllTabStates = useCallback((): Map<string, State> => {
    return new Map(statesRef.current);
  }, []);

  // The active tab's current state, with a stable identity for cancel().
  const activeState = activeTabId
    ? getOrCreateState(statesRef.current, activeTabId)
    : idleReady
      ? { ...initialState, meta: idleMeta }
      : initialState;
  const stateRef = useRef(activeState);
  stateRef.current = activeState;

  // Dispatch to a specific tab's state. If the tab doesn't have state yet, it's
  // created. Bumps the version so React re-renders when it becomes active.
  const dispatchTo = useCallback((tabId: string, action: Action) => {
    const states = statesRef.current;
    const prev = getOrCreateState(states, tabId);
    const next = reducer(prev, action);
    if (prev !== next) {
      states.set(tabId, next);
      bump();
    }
  }, [bump]);

  const hydrateInflightRef = useRef(new Map<string, Promise<void>>());

  const loadSessionDataForTab = useCallback(async (tabId: string, reset = false) => {
    const inflight = hydrateInflightRef.current.get(tabId);
    if (inflight) return inflight;

    const work = (async () => {
      try {
        if (reset) dispatchTo(tabId, { type: "reset" });
        setBootPhase(t("boot.loadingWorkspace"));
        const metaP = withIPCTimeout(app.MetaForTab(tabId), IPC_META_TIMEOUT_MS, "MetaForTab");
        const historyP = withIPCTimeout(app.HistoryForTab(tabId), IPC_HYDRATE_TIMEOUT_MS, "HistoryForTab");
        const [meta, history] = await Promise.all([metaP, historyP]);
        const existingBalance = statesRef.current.get(tabId)?.balance;
        dispatchTo(tabId, {
          type: "hydrate",
          meta,
          context: statesRef.current.get(tabId)?.context ?? { used: 0, window: 0 },
          effort: statesRef.current.get(tabId)?.effort ?? { supported: false, current: "", default: "", levels: [] },
          balance: existingBalance ?? { available: false, display: "" },
          jobs: statesRef.current.get(tabId)?.jobs ?? [],
          checkpoints: statesRef.current.get(tabId)?.checkpoints ?? [],
          messages: asArray(history),
        });
        if (meta.ready || meta.startupErr) {
          setBootPhase(null);
        } else {
          setBootPhase(t("boot.startingAgent"));
        }
        void Promise.all([
          withIPCTimeout(app.ContextUsageForTab(tabId), IPC_META_TIMEOUT_MS, "ContextUsageForTab"),
          withIPCTimeout(app.EffortForTab(tabId), IPC_META_TIMEOUT_MS, "EffortForTab"),
          withIPCTimeout(app.JobsForTab(tabId), IPC_META_TIMEOUT_MS, "JobsForTab"),
          withIPCTimeout(app.CheckpointsForTab(tabId), IPC_META_TIMEOUT_MS, "CheckpointsForTab"),
        ])
          .then(([context, effort, jobs, checkpoints]) => {
            dispatchTo(tabId, { type: "context", context });
            dispatchTo(tabId, { type: "effort", effort });
            dispatchTo(tabId, { type: "jobs", jobs: asArray(jobs) });
            dispatchTo(tabId, { type: "checkpoints", checkpoints: asArray(checkpoints) });
          })
          .catch((err) => logBridgeError(`hydrate.details(${tabId})`, err));
        void withIPCTimeout(app.BalanceForTab(tabId), IPC_BALANCE_TIMEOUT_MS, "BalanceForTab")
          .then((balance) => dispatchTo(tabId, { type: "balance", balance }))
          .catch((err) => logBridgeError(`BalanceForTab(${tabId})`, err));
      } catch (err) {
        logBridgeError(`loadSessionDataForTab(${tabId})`, err);
        setBootPhase(null);
        dispatchTo(tabId, {
          type: "meta",
          meta: {
            label: "",
            ready: true,
            startupErr: toErrorMessage(err, t("boot.hydrateFailed")),
            eventChannel: "agent:event",
            cwd: "",
            bypass: false,
          },
        });
      }
    })();

    hydrateInflightRef.current.set(tabId, work);
    try {
      await work;
    } finally {
      hydrateInflightRef.current.delete(tabId);
    }
  }, [dispatchTo]);

  const activeTabFromBackend = useCallback(async (): Promise<TabMeta | undefined> => {
    const tabs = asArray(
      await app.ListTabs().catch((err) => {
        logBridgeError("ListTabs", err);
        return [] as TabMeta[];
      }),
    );
    return tabs.find((tab) => tab.active) ?? tabs[0];
  }, []);

  const pollTabAgentReady = useCallback(async (tabId: string): Promise<Meta | undefined> => {
    try {
      return await withIPCTimeout(app.MetaForTab(tabId), IPC_META_TIMEOUT_MS, "MetaForTab");
    } catch (err) {
      logBridgeError(`MetaForTab(${tabId})`, err);
      return undefined;
    }
  }, []);

  const rememberTabTitles = useCallback((tabs: TabMeta[]) => {
    for (const tab of tabs) {
      const title = tab.topicTitle?.trim() || tab.workspaceName?.trim();
      if (title) tabTitlesRef.current.set(tab.id, title);
    }
  }, []);

  const markIdleReady = useCallback(() => {
    setBootPhase(null);
    setIdleReady(true);
    bump();
  }, [bump]);

  const syncActiveTabFromBackend = useCallback(async (reset = false): Promise<string | undefined> => {
    const tabs = asArray(
      await app.ListTabs().catch((err) => {
        logBridgeError("ListTabs", err);
        return [] as TabMeta[];
      }),
    );
    if (tabs.length === 0) {
      setActiveTabId(undefined);
      markIdleReady();
      return undefined;
    }
    const active = tabs.find((tab) => tab.active) ?? tabs[0];
    if (!active) {
      markIdleReady();
      return undefined;
    }
    setIdleReady(false);
    setActiveTabId(active.id);
    rememberTabTitles([active]);
    await loadSessionDataForTab(active.id, reset);
    return active.id;
  }, [loadSessionDataForTab, markIdleReady, rememberTabTitles]);

  const activeTabIdRef = useRef(activeTabId);
  activeTabIdRef.current = activeTabId;

  const streamBufRef = useRef<{ tabId: string; text: string; reasoning: string } | null>(null);
  const streamRafRef = useRef<number>(0);

  const flushStreamBuf = useCallback(() => {
    streamRafRef.current = 0;
    const buf = streamBufRef.current;
    if (!buf) return;
    streamBufRef.current = null;
    if (!buf.text && !buf.reasoning) return;
    dispatchTo(buf.tabId, {
      type: "stream_delta",
      text: buf.text || undefined,
      reasoning: buf.reasoning || undefined,
    });
  }, [dispatchTo]);

  const loadSessionData = useCallback(async () => {
    if (activeTabIdRef.current) {
      const tabId = activeTabIdRef.current;
      const existing = statesRef.current.get(tabId);
      if (existing?.meta?.ready === true && !existing.meta.startupErr) {
        return;
      }
      await loadSessionDataForTab(tabId);
      return;
    }
    await syncActiveTabFromBackend();
  }, [loadSessionDataForTab, syncActiveTabFromBackend]);

  useEffect(() => {
    const off = onEvent((e) => {
      const targetTabId = e.tabId || activeTabIdRef.current;
      if (!targetTabId) return;

      if (e.kind === "text" || e.kind === "reasoning") {
        let buf = streamBufRef.current;
        if (!buf || buf.tabId !== targetTabId) {
          flushStreamBuf();
          buf = { tabId: targetTabId, text: "", reasoning: "" };
        }
        if (e.kind === "text") buf.text += e.text ?? "";
        else buf.reasoning += e.reasoning ?? "";
        streamBufRef.current = buf;
        if (!streamRafRef.current) {
          streamRafRef.current = requestAnimationFrame(flushStreamBuf);
        }
        return;
      }

      flushStreamBuf();
      dispatchTo(targetTabId, { type: "event", e });
      if (e.kind === "turn_started" || e.kind === "turn_done") {
        notifyTabMetasChanged();
      }
      if (
        (e.kind === "approval_request" || e.kind === "ask_request") &&
        targetTabId &&
        targetTabId !== activeTabIdRef.current
      ) {
        const tabTitle = tabTitlesRef.current.get(targetTabId) ?? targetTabId;
        notifyBackgroundTabDecision(tabTitle, e.approval, e.ask, {
          titleApproval: t("decision.notifyApproval"),
          titleAsk: t("decision.notifyAsk"),
          bodyApproval: (tool) => t("decision.pendingApproval", { tool }),
          bodyAsk: t("decision.pendingAsk"),
        });
      }
      if (e.kind === "usage" && e.usage) {
        recordUsageActivity(e.usage);
      }
      if (e.kind === "compaction_done" && e.compaction?.summary) {
        recordCompactionActivity(e.compaction.trigger);
      }
      if (e.kind === "turn_done") {
        turnWatchdogFiredRef.current.delete(targetTabId);
        turnWatchdogForceClearedRef.current.delete(targetTabId);
        if (!e.err) {
          window.setTimeout(() => {
            dispatchTo(targetTabId, { type: "clearRecentCompletion" });
          }, 12000);
        }
        void Promise.all([
          app.ContextUsageForTab(targetTabId),
          app.BalanceForTab(targetTabId),
          app.EffortForTab(targetTabId),
          app.JobsForTab(targetTabId),
        ]).then(([context, balance, effort, jobs]) => {
          dispatchTo(targetTabId, { type: "context", context });
          dispatchTo(targetTabId, { type: "balance", balance });
          dispatchTo(targetTabId, { type: "effort", effort });
          dispatchTo(targetTabId, { type: "jobs", jobs: asArray(jobs) });
        }).catch((err) => logBridgeError("turn_done.refresh", err));
      }
    });

    return () => {
      off();
      if (streamRafRef.current) cancelAnimationFrame(streamRafRef.current);
      flushStreamBuf();
    };
  }, [dispatchTo, flushStreamBuf]);

  useEffect(() => {
    const offReady = onReady(() => {
      void loadSessionData();
    });
    const offShell = onTabsShell(() => {
      void syncActiveTabFromBackend();
    });
    return () => {
      offReady();
      offShell();
    };
  }, [loadSessionData, syncActiveTabFromBackend]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      if (activeTabIdRef.current) return;
      for (let attempt = 0; attempt < BOOT_READY_MAX_POLLS && !cancelled; attempt += 1) {
        const tabs = asArray(
          await app.ListTabs().catch(() => [] as TabMeta[]),
        );
        if (tabs.length === 0) {
          markIdleReady();
          return;
        }
        const id = await syncActiveTabFromBackend();
        if (id) return;
        setBootPhase(t("boot.loadingWorkspace"));
        await new Promise((resolve) => window.setTimeout(resolve, BOOT_READY_POLL_MS));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [markIdleReady, syncActiveTabFromBackend]);

  useEffect(() => {
    if (!activeTabId) return;
    void loadSessionDataForTab(activeTabId);
  }, [activeTabId, loadSessionDataForTab]);

  useEffect(() => {
    if (!activeTabId) return;

    let cancelled = false;
    void (async () => {
      for (let attempt = 0; attempt < BOOT_READY_MAX_POLLS && !cancelled; attempt += 1) {
        const current = statesRef.current.get(activeTabId);
        if (current?.meta?.ready === true || current?.meta?.startupErr) {
          setBootPhase(null);
          return;
        }

        const meta = await pollTabAgentReady(activeTabId);
        if (cancelled) return;
        if (meta?.ready || meta?.startupErr) {
          await loadSessionDataForTab(activeTabId);
          setBootPhase(null);
          return;
        }

        setBootPhase(t("boot.startingAgent"));
        await new Promise((resolve) => window.setTimeout(resolve, BOOT_READY_POLL_MS));
      }

      if (cancelled) return;
      setBootPhase(null);
      dispatchTo(activeTabId, {
        type: "meta",
        meta: {
          ...(statesRef.current.get(activeTabId)?.meta ?? {
            label: "",
            eventChannel: "agent:event",
            cwd: "",
            bypass: false,
          }),
          ready: true,
          startupErr: t("boot.timeout"),
        },
      });
    })();

    return () => {
      cancelled = true;
    };
  }, [activeTabId, dispatchTo, loadSessionDataForTab, pollTabAgentReady]);

  useEffect(() => {
    const id = window.setInterval(() => {
      const now = Date.now();
      for (const [tabId, s] of statesRef.current.entries()) {
        const fired = turnWatchdogFiredRef.current.get(tabId);
        if (shouldEmitTurnWatchdogNotice(s, now, fired)) {
          turnWatchdogFiredRef.current.set(tabId, s.turnStartAt);
          dispatchTo(tabId, {
            type: "local_notice",
            level: "warn",
            text: t("turnWatchdog.stillWaiting"),
          });
        }
        if (shouldForceClearTurnWatchdog(s, now, turnWatchdogForceClearedRef.current.get(tabId))) {
          turnWatchdogForceClearedRef.current.set(tabId, s.turnStartAt);
          app.CancelTab(tabId).catch((err) => logBridgeError("watchdog.cancel", err));
          dispatchTo(tabId, {
            type: "event",
            e: { kind: "turn_done", err: t("turnWatchdog.timeout") },
          });
        }
      }
    }, TURN_WATCHDOG_POLL_MS);
    return () => window.clearInterval(id);
  }, [dispatchTo]);

  const notice = useCallback((text: string, level: "info" | "warn" = "info") => {
    if (!activeTabId) return;
    dispatchTo(activeTabId, { type: "local_notice", level, text });
  }, [activeTabId, dispatchTo]);

  const reportFailure = useCallback((err: unknown) => {
    if (!activeTabId) return;
    const msg = toErrorMessage(err);
    dispatchTo(activeTabId, {
      type: "local_notice",
      level: "warn",
      text: t("common.operationFailed", { msg: msg || "unknown" }),
    });
  }, [activeTabId, dispatchTo]);

  const send = useCallback((displayText: string, submitText = displayText) => {
    if (!activeTabId) return;
    const cur = stateRef.current;
    if (shouldBlockConcurrentSend(cur)) {
      notice(t("composer.agentBusy"), "warn");
      return;
    }
    dispatchTo(activeTabId, { type: "user", text: displayText });
    const display = displayText.trim(); const submit = submitText.trim();
    (display !== submit ? app.SubmitDisplayToTab(activeTabId, display, submit) : app.SubmitToTab(activeTabId, submit)).catch((err) => {
      dispatchTo(activeTabId, { type: "unsend" });
      reportFailure(err);
    });
  }, [activeTabId, dispatchTo, reportFailure]);

  const runShell = useCallback((command: string) => {
    if (!activeTabId) return;
    dispatchTo(activeTabId, { type: "user", text: `!${command}` });
    app.RunShell(command).catch(reportFailure);
  }, [activeTabId, dispatchTo, reportFailure]);

  const cancel = useCallback((): string | undefined => {
    const cur = stateRef.current;
    const tabId = activeTabId;
    if (cur.running && cur.pendingUser !== undefined) {
      const text = cur.pendingUser;
      if (tabId) {
        dispatchTo(tabId, { type: "unsend" });
        app.CancelTab(tabId).catch(reportFailure);
      }
      return text;
    }
    if (tabId) app.CancelTab(tabId).catch(reportFailure);
    return undefined;
  }, [activeTabId, dispatchTo, reportFailure]);

  const approve = useCallback((id: string, allow: boolean, session: boolean, persist: boolean) => {
    if (!activeTabId) return;
    dispatchTo(activeTabId, { type: "clearApproval" });
    app.ApproveTab(activeTabId, id, allow, session, persist).catch(reportFailure);
  }, [activeTabId, dispatchTo, reportFailure]);

  const answerQuestion = useCallback((id: string, answers: QuestionAnswer[]) => {
    if (!activeTabId) return;
    const dismissed = answers.length === 0 || answers.every((a) => (a.selected?.length ?? 0) === 0);
    dispatchTo(activeTabId, { type: "resolveAsk", id, answers, dismissed });
    app.AnswerQuestionForTab(activeTabId, id, answers).catch(reportFailure);
  }, [activeTabId, dispatchTo, reportFailure]);

  const setControllerMode = useCallback((mode: "plan" | "yolo" | "normal"): Promise<void> => {
    return app.SetModeForTab(activeTabId ?? "", mode).then(() => {
      if (mode === "yolo" && activeTabId) dispatchTo(activeTabId, { type: "clearApproval" });
    }).catch((err) => {
      reportFailure(err);
    });
  }, [activeTabId, dispatchTo, reportFailure]);

  const newSession = useCallback(async () => {
    try {
      await app.NewSession();
    } catch (err) {
      reportFailure(err);
      throw err;
    }
    if (activeTabId) dispatchTo(activeTabId, { type: "reset" });
  }, [activeTabId, dispatchTo, reportFailure]);

  const listSessions = useCallback(async (): Promise<SessionMeta[]> => asArray<SessionMeta>(await app.ListSessions().catch(() => [])), []);
  const listTrashedSessions = useCallback(async (): Promise<SessionMeta[]> => asArray<SessionMeta>(await app.ListTrashedSessions().catch(() => [])), []);
  const resumeSession = useCallback(async (path: string, tabId?: string) => {
    const targetTabId = tabId || activeTabId;
    if (!targetTabId) return;
    if (tabId) {
      for (let attempt = 0; attempt < BOOT_READY_MAX_POLLS; attempt += 1) {
        const meta = await pollTabAgentReady(tabId);
        if (meta?.ready || meta?.startupErr) break;
        await new Promise((resolve) => window.setTimeout(resolve, BOOT_READY_POLL_MS));
      }
    }
    const messages = asArray(
      await (tabId ? app.ResumeSessionForTab(tabId, path) : app.ResumeSession(path)).catch((err) => {
        reportFailure(err);
        return [] as HistoryMessage[];
      }),
    );
    dispatchTo(targetTabId, { type: "reset" });
    if (messages.length) dispatchTo(targetTabId, { type: "history", messages });
    app.ContextUsageForTab(targetTabId).then((context) => dispatchTo(targetTabId, { type: "context", context })).catch((err) => logBridgeError("ContextUsageForTab", err));
  }, [activeTabId, dispatchTo, pollTabAgentReady, reportFailure]);

  const previewSession = useCallback(async (path: string): Promise<HistoryMessage[]> => asArray<HistoryMessage>(await app.PreviewSession(path).catch(() => [])), []);
  const deleteSession = useCallback((path: string) => app.DeleteSession(path).catch(reportFailure), [reportFailure]);
  const restoreSession = useCallback((path: string) => app.RestoreSession(path).catch(reportFailure), [reportFailure]);
  const purgeTrashedSession = useCallback((path: string) => app.PurgeTrashedSession(path).catch(reportFailure), [reportFailure]);
  const renameSession = useCallback((path: string, title: string) => app.RenameSession(path, title).catch(reportFailure), [reportFailure]);

  const refreshMeta = useCallback(async () => {
    if (!activeTabId) return;
    try {
      dispatchTo(activeTabId, { type: "meta", meta: await app.MetaForTab(activeTabId) });
      dispatchTo(activeTabId, { type: "context", context: await app.ContextUsageForTab(activeTabId) });
      dispatchTo(activeTabId, { type: "effort", effort: await app.EffortForTab(activeTabId) });
    } catch (err) {
      logBridgeError("refreshMeta", err);
    }
  }, [activeTabId, dispatchTo, reportFailure]);

  const refreshWorkspaceState = useCallback(async (path: string): Promise<string> => {
    if (path) {
      const active = await activeTabFromBackend();
      const reset = !(active?.workspaceRoot && sameWorkspaceRoot(active.workspaceRoot, path));
      await syncActiveTabFromBackend(reset);
    }
    return path;
  }, [activeTabFromBackend, syncActiveTabFromBackend]);

  const pickWorkspace = useCallback(async (): Promise<string> => {
    const path = await app.PickWorkspace().catch(() => "");
    return refreshWorkspaceState(path);
  }, [refreshWorkspaceState]);
  const switchWorkspace = useCallback(async (path: string): Promise<string> => {
    const next = await app.SwitchWorkspace(path).catch((err) => {
      reportFailure(err);
      return "";
    });
    return refreshWorkspaceState(next);
  }, [refreshWorkspaceState, reportFailure]);

  const compact = useCallback(() => { app.Compact().catch(reportFailure); }, [reportFailure]);

  const setModel = useCallback(async (name: string) => {
    if (!activeTabId) return;
    try {
      await app.SetModelForTab(activeTabId, name);
    } catch (err) {
      reportFailure(err);
      try {
        dispatchTo(activeTabId, { type: "meta", meta: await app.MetaForTab(activeTabId) });
      } catch (refreshErr) {
        logBridgeError("setModel.metaAfterError", refreshErr);
      }
      return;
    }
    try {
      dispatchTo(activeTabId, { type: "meta", meta: await app.MetaForTab(activeTabId) });
      dispatchTo(activeTabId, { type: "context", context: await app.ContextUsageForTab(activeTabId) });
      dispatchTo(activeTabId, { type: "effort", effort: await app.EffortForTab(activeTabId) });
    } catch (err) {
      logBridgeError("setModel.refresh", err);
    }
  }, [activeTabId, dispatchTo, reportFailure]);

  const setEffort = useCallback(async (level: string) => {
    if (!activeTabId) return;
    try {
      await app.SetEffortForTab(activeTabId, level);
    } catch (err) {
      reportFailure(err);
      return;
    }
    try {
      dispatchTo(activeTabId, { type: "meta", meta: await app.MetaForTab(activeTabId) });
      dispatchTo(activeTabId, { type: "context", context: await app.ContextUsageForTab(activeTabId) });
      dispatchTo(activeTabId, { type: "effort", effort: await app.EffortForTab(activeTabId) });
    } catch (err) {
      logBridgeError("setEffort.refresh", err);
    }
  }, [activeTabId, dispatchTo, reportFailure]);

  const fetchMemory = useCallback((): Promise<MemoryView> =>
    app.Memory().catch(() => ({ docs: [], facts: [], scopes: [], storeDir: "", available: false })), []);
  const remember = useCallback(async (scope: string, note: string) => {
    try {
      await app.Remember(scope, note);
    } catch (err) {
      reportFailure(err);
    }
  }, [reportFailure]);
  const forget = useCallback(async (name: string) => {
    try {
      await app.Forget(name);
    } catch (err) {
      reportFailure(err);
    }
  }, [reportFailure]);
  const saveDoc = useCallback(async (path: string, body: string) => {
    try {
      await app.SaveDoc(path, body);
    } catch (err) {
      reportFailure(err);
    }
  }, [reportFailure]);

  const rewind = useCallback(async (turn: number, scope: string) => {
    const sourceTabId = activeTabId;
    if (!sourceTabId) return;
    const actionScope = (["fork", "summ-from", "summ-upto", "conversation", "code", "both"].includes(scope) ? scope : "both") as MessageActionScope;
    dispatchTo(sourceTabId, { type: "message_action_start", action: { turn, scope: actionScope } });
    dispatchTo(sourceTabId, { type: "local_notice", level: "info", text: messageActionBusyText(actionScope) });
    try {
      if (actionScope === "fork") {
        const tab = await app.Fork(turn);
        if (tab?.id) {
          setActiveTabId(tab.id);
          await loadSessionDataForTab(tab.id, true);
        } else {
          await syncActiveTabFromBackend(true);
        }
        return;
      }

      if (actionScope === "summ-from") await app.SummarizeFrom(turn);
      else if (actionScope === "summ-upto") await app.SummarizeUpTo(turn);
      else await app.Rewind(turn, actionScope);

      const messages = asArray(await app.HistoryForTab(sourceTabId).catch(() => [] as HistoryMessage[]));
      dispatchTo(sourceTabId, { type: "reset" });
      if (messages.length) dispatchTo(sourceTabId, { type: "history", messages });
      dispatchTo(sourceTabId, { type: "context", context: await app.ContextUsageForTab(sourceTabId) });
      dispatchTo(sourceTabId, { type: "checkpoints", checkpoints: asArray(await app.CheckpointsForTab(sourceTabId)) });
    } catch {
      /* The controller emits a warning notice with the specific failure reason. */
    } finally {
      dispatchTo(sourceTabId, { type: "message_action_done" });
    }
  }, [activeTabId, dispatchTo, loadSessionDataForTab, syncActiveTabFromBackend]);

  // Tab management: switch preserves per-tab state; open creates it.
  const switchTab = useCallback(async (tabId: string) => {
    try {
      await app.SetActiveTab(tabId);
      setIdleReady(false);
      setActiveTabId(tabId);
      // Load session data into the tab's state if it hasn't been loaded yet.
      const states = statesRef.current;
      if (!states.has(tabId) || !states.get(tabId)?.meta) {
        await loadSessionDataForTab(tabId);
      }
      notifyTabMetasChanged();
    } catch (err) {
      logBridgeError("switchTab", err);
    }
  }, [loadSessionDataForTab]);

  const openProjectTab = useCallback(async (workspaceRoot: string, topicId: string, reload = false, freshSession = false): Promise<TabMeta | undefined> => {
    try {
      const meta = freshSession
        ? await app.OpenProjectTabFresh(workspaceRoot, topicId)
        : await app.OpenProjectTab(workspaceRoot, topicId);
      rememberTabTitles([meta]);
      await app.SetActiveTab(meta.id).catch((err) => logBridgeError("SetActiveTab", err));
      setIdleReady(false);
      setActiveTabId(meta.id);
      const cached = statesRef.current.get(meta.id);
      if (reload || !cached?.meta) {
        await loadSessionDataForTab(meta.id, reload);
      }
      notifyTabMetasChanged();
      return meta;
    } catch (err) {
      logBridgeError("openProjectTab", err);
      return undefined;
    }
  }, [loadSessionDataForTab, rememberTabTitles]);

  const openGlobalTab = useCallback(async (topicId: string, reload = false, freshSession = false): Promise<TabMeta | undefined> => {
    try {
      const meta = freshSession
        ? await app.OpenGlobalTabFresh(topicId)
        : await app.OpenGlobalTab(topicId);
      rememberTabTitles([meta]);
      await app.SetActiveTab(meta.id).catch((err) => logBridgeError("SetActiveTab", err));
      setIdleReady(false);
      setActiveTabId(meta.id);
      const cached = statesRef.current.get(meta.id);
      if (reload || !cached?.meta) {
        await loadSessionDataForTab(meta.id, reload);
      }
      notifyTabMetasChanged();
      return meta;
    } catch (err) {
      logBridgeError("openGlobalTab", err);
      return undefined;
    }
  }, [loadSessionDataForTab, rememberTabTitles]);

  const closeTab = useCallback(async (tabId: string) => {
    try {
      await app.CloseTab(tabId);
      statesRef.current.delete(tabId);
      tabTitlesRef.current.delete(tabId);
      bump();
      if (tabId === activeTabId) {
        const tabs = asArray(await app.ListTabs().catch(() => [] as TabMeta[]));
        if (tabs.length === 0) {
          setActiveTabId(undefined);
          markIdleReady();
        } else {
          await syncActiveTabFromBackend(true);
        }
      }
      notifyTabMetasChanged();
    } catch (err) {
      logBridgeError("closeTab", err);
    }
  }, [activeTabId, bump, markIdleReady, syncActiveTabFromBackend]);

  const reorderTabs = useCallback(async (tabIds: string[]) => {
    try {
      await app.ReorderTabs(tabIds);
      notifyTabMetasChanged();
    } catch (err) {
      logBridgeError("reorderTabs", err);
    }
  }, []);

  return {
    state: activeState,
    activeTabId,
    bootPhase,
    send, runShell, notice, cancel, approve, answerQuestion, setControllerMode,
    newSession, listSessions, listTrashedSessions, resumeSession, previewSession, deleteSession, restoreSession, purgeTrashedSession, renameSession,
    refreshMeta, pickWorkspace, switchWorkspace, compact, rewind, setModel, setEffort,
    fetchMemory, remember, forget, saveDoc,
    switchTab, openProjectTab, openGlobalTab, closeTab, reorderTabs,
    syncActiveTab: syncActiveTabFromBackend,
    getAllTabStates,
    rememberTabTitles,
  };
}

export {
  applyEvent as controllerApplyWireEvent,
  initialState as controllerInitialState,
  reducer as controllerReducer,
  shouldBlockConcurrentSend,
  shouldArmTurnWatchdog,
  shouldEmitTurnWatchdogNotice,
  shouldForceClearTurnWatchdog,
  isStaleStreamDoneErr,
  TURN_WATCHDOG_MS,
  TURN_WATCHDOG_FORCE_CLEAR_MS,
};
export type { State as ControllerState, Action as ControllerAction };
