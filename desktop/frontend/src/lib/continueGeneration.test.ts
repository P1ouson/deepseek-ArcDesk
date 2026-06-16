import { describe, expect, it } from "vitest";
import {
  controllerApplyWireEvent,
  controllerInitialState,
  controllerReducer,
} from "./useController";
import { isResponseTruncationNotice, truncatedAssistantIds } from "./responseTruncation";
import { isContinueTextStream } from "./continueGeneration";
import { isCancellableAgentWork, shouldBlockAgentSend } from "./agentActivity";

describe("responseTruncation", () => {
  it("detects max-token truncation notices", () => {
    expect(isResponseTruncationNotice("response truncated: hit max output tokens")).toBe(true);
    expect(isResponseTruncationNotice("all good")).toBe(false);
  });

  it("maps truncated assistant ids from notices", () => {
    const items = [
      { kind: "user" as const, id: "u1", text: "hi" },
      { kind: "assistant" as const, id: "a1", text: "partial", reasoning: "", streaming: false },
      { kind: "notice" as const, id: "n1", level: "warn" as const, text: "response truncated: hit max output tokens" },
    ];
    expect(truncatedAssistantIds(items)).toEqual(new Set(["a1"]));
  });
});

describe("continue generation", () => {
  it("reuses the same assistant bubble while continuing", () => {
    let state = controllerApplyWireEvent(controllerInitialState, { kind: "turn_started" });
    state = controllerApplyWireEvent(state, { kind: "text", text: "partial" });
    state = controllerApplyWireEvent(state, { kind: "turn_done" });
    const assistantId = state.items.find((it) => it.kind === "assistant")?.id;
    expect(assistantId).toBeTruthy();

    state = controllerReducer(state, {
      type: "continue_generation",
      assistantId: assistantId!,
      text: "partial",
      reasoning: "",
    });
    state = controllerApplyWireEvent(state, { kind: "turn_started" });
    state = controllerApplyWireEvent(state, { kind: "text", text: " more" });
    expect(state.live?.text).toBe("partial more");
    expect(state.items.filter((it) => it.kind === "assistant")).toHaveLength(1);
    state = controllerApplyWireEvent(state, { kind: "turn_done" });
    expect(state.items.find((it) => it.kind === "assistant")?.text).toBe("partial more");
    expect(state.continueActive).toBe(false);
  });

  it("keeps composer send enabled during continue text stream", () => {
    let state = controllerApplyWireEvent(controllerInitialState, { kind: "turn_started" });
    state = controllerApplyWireEvent(state, { kind: "text", text: "partial" });
    state = controllerApplyWireEvent(state, { kind: "turn_done" });
    const assistantId = state.items.find((it) => it.kind === "assistant")?.id!;

    state = controllerReducer(state, {
      type: "continue_generation",
      assistantId,
      text: "partial",
      reasoning: "",
    });
    expect(isContinueTextStream(state)).toBe(true);
    expect(shouldBlockAgentSend(state)).toBe(false);
    expect(isCancellableAgentWork(state)).toBe(false);
  });
});
