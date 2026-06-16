import type { State } from "./useController";

/** Continue-only text stream: composer stays on Send, not Stop. */
export function isContinueTextStream(
  state: Pick<
    State,
    "continueActive" | "running" | "turnActive" | "live" | "pendingUser" | "approval" | "ask" | "items"
  >,
): boolean {
  if (!state.continueActive || !state.running) return false;
  if (state.pendingUser || state.approval || state.ask) return false;

  const runningTools = state.items.filter(
    (it): it is Extract<(typeof state.items)[number], { kind: "tool" }> =>
      it.kind === "tool" && it.status === "running",
  );
  if (runningTools.length > 0) return false;

  return Boolean(state.live || state.turnActive);
}
