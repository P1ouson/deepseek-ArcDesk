import { describe, expect, it } from "vitest";
import {
  controllerApplyWireEvent,
  controllerInitialState,
  controllerReducer,
  shouldBlockConcurrentSend,
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

  it("reverts optimistic send when backend emits agent busy notice", () => {
    let state = controllerReducer(controllerInitialState, { type: "user", text: "queued" });
    state = controllerApplyWireEvent(state, { kind: "notice", level: "warn", text: "Agent is still working on the previous message" });
    expect(state.pendingUser).toBeUndefined();
    expect(state.items.some((it) => it.kind === "user" && it.text === "queued")).toBe(false);
  });
});
