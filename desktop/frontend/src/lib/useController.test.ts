import { describe, expect, it } from "vitest";
import {
  controllerApplyWireEvent,
  controllerInitialState,
  controllerReducer,
  shouldArmTurnWatchdog,
  shouldBlockConcurrentSend,
  shouldEmitTurnWatchdogNotice,
  TURN_WATCHDOG_MS,
} from "./useController";

describe("useController reducer", () => {
  it("queues user text on send", () => {
    const next = controllerReducer(controllerInitialState, { type: "user", text: "hello" });
    expect(next.pendingUser).toBe("hello");
    expect(next.running).toBe(true);
  });

  it("streams assistant text from wire events", () => {
    let state = controllerReducer(controllerInitialState, { type: "event", e: { kind: "turn_started" } });
    state = controllerApplyWireEvent(state, { kind: "text", text: "hi" });
    expect(state.live?.text).toBe("hi");
  });

  it("blocks concurrent send while a turn is active", () => {
    let state = controllerReducer(controllerInitialState, { type: "user", text: "first" });
    state = controllerApplyWireEvent(state, { kind: "turn_started" });
    expect(shouldBlockConcurrentSend(state)).toBe(true);
  });

  it("reverts optimistic send when backend emits agent_busy code notice", () => {
    let state = controllerReducer(controllerInitialState, { type: "user", text: "queued" });
    state = controllerApplyWireEvent(state, {
      kind: "notice",
      level: "warn",
      code: "agent_busy",
      text: "Agent is still working on the previous message",
    });
    expect(state.pendingUser).toBeUndefined();
    expect(state.items.some((it) => it.kind === "user" && it.text === "queued")).toBe(false);
    expect(state.items.some((it) => it.kind === "notice")).toBe(true);
  });

  it("does not revert pending user on unrelated warn notice", () => {
    let state = controllerReducer(controllerInitialState, { type: "user", text: "queued" });
    state = controllerApplyWireEvent(state, {
      kind: "notice",
      level: "warn",
      text: "Provider rate limited — retrying shortly",
    });
    expect(state.pendingUser).toBeUndefined();
    expect(state.items.some((it) => it.kind === "user" && it.text === "queued")).toBe(true);
    expect(state.items.some((it) => it.kind === "notice")).toBe(true);
  });

  it("reverts optimistic send via legacy English text when code is absent", () => {
    let state = controllerReducer(controllerInitialState, { type: "user", text: "queued" });
    state = controllerApplyWireEvent(state, {
      kind: "notice",
      level: "warn",
      text: "Agent is still working on the previous message",
    });
    expect(state.pendingUser).toBeUndefined();
    expect(state.items.some((it) => it.kind === "user" && it.text === "queued")).toBe(false);
  });

  it("concurrent guard leaves no user bubble after agent_busy notice", () => {
    let state = controllerReducer(controllerInitialState, { type: "user", text: "second" });
    state = controllerApplyWireEvent(state, {
      kind: "notice",
      level: "warn",
      code: "agent_busy",
      text: "busy",
    });
    expect(state.items.filter((it) => it.kind === "user")).toHaveLength(0);
  });
});

describe("turn watchdog", () => {
  const activeTurn = {
    running: true,
    turnActive: true,
    turnStartAt: 1_000,
    retry: undefined as { attempt: number; max: number } | undefined,
  };

  it("does not emit before timeout", () => {
    expect(shouldEmitTurnWatchdogNotice(activeTurn, 1_000 + TURN_WATCHDOG_MS - 1, undefined)).toBe(false);
  });

  it("emits after timeout when turn is still active", () => {
    expect(shouldEmitTurnWatchdogNotice(activeTurn, 1_000 + TURN_WATCHDOG_MS, undefined)).toBe(true);
  });

  it("emits only once per turnStartAt", () => {
    const now = 1_000 + TURN_WATCHDOG_MS;
    expect(shouldEmitTurnWatchdogNotice(activeTurn, now, activeTurn.turnStartAt)).toBe(false);
  });

  it("does not arm while retrying", () => {
    expect(shouldArmTurnWatchdog({ ...activeTurn, retry: { attempt: 1, max: 3 } })).toBe(false);
    expect(shouldEmitTurnWatchdogNotice(
      { ...activeTurn, retry: { attempt: 1, max: 3 } },
      1_000 + TURN_WATCHDOG_MS,
      undefined,
    )).toBe(false);
  });

  it("resets eligibility after turn_done clears running/turnActive", () => {
    let state = controllerApplyWireEvent(
      { ...controllerInitialState, running: true, turnActive: true, turnStartAt: Date.now() },
      { kind: "turn_done" },
    );
    expect(state.running).toBe(false);
    expect(state.turnActive).toBe(false);
    expect(shouldArmTurnWatchdog(state)).toBe(false);
  });
});
