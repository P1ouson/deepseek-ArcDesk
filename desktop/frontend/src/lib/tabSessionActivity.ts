import type { ControllerState } from "./useController";
import type { TabMeta } from "./types";

export type TabPulse = "none" | "running" | "completed";

export type TabAttention = {
  tabId: string;
  topicTitle: string;
  workspaceName: string;
  running: boolean;
  needsDecision: boolean;
  pulse: TabPulse;
  decisionLabel?: string;
};

export function tabIsAgentRunning(state: Pick<ControllerState, "running" | "turnActive">): boolean {
  return state.running || state.turnActive;
}

export function tabNeedsDecision(state: Pick<ControllerState, "approval" | "ask">): boolean {
  return state.approval != null || state.ask != null;
}

export function tabPulseForState(
  state: Pick<ControllerState, "running" | "turnActive" | "recentlyCompleted"> | undefined,
  backendRunning = false,
): TabPulse {
  if (state ? tabIsAgentRunning(state) : backendRunning) return "running";
  return "completed";
}

export function decisionLabelForTab(
  state: Pick<ControllerState, "approval" | "ask">,
  labels: { plan: string; approval: (tool: string) => string; ask: string },
): string | undefined {
  if (state.approval) {
    if (state.approval.tool === "exit_plan_mode") return labels.plan;
    return labels.approval(state.approval.tool);
  }
  if (state.ask) return labels.ask;
  return undefined;
}

/** Merge per-tab reducer state with backend tab metas for the open-tab strip. */
export function listTabAttention(
  tabMetas: TabMeta[],
  stateByTabId: Map<string, ControllerState>,
  labels: { plan: string; approval: (tool: string) => string; ask: string },
): TabAttention[] {
  return tabMetas.map((tab) => {
    const local = stateByTabId.get(tab.id);
    const running = local ? tabIsAgentRunning(local) : tab.running;
    const needsDecision = local ? tabNeedsDecision(local) : false;
    return {
      tabId: tab.id,
      topicTitle: tab.topicTitle?.trim() || tab.workspaceName || tab.id,
      workspaceName: tab.workspaceName?.trim() || tab.workspaceRoot || "",
      running,
      needsDecision,
      pulse: tabPulseForState(local, tab.running),
      decisionLabel: local ? decisionLabelForTab(local, labels) : undefined,
    };
  });
}

export function countBackgroundAttention(attention: TabAttention[], activeTabId?: string): number {
  return attention.filter((row) => row.tabId !== activeTabId && (row.running || row.needsDecision)).length;
}

/** Every sidebar-opened / persisted workspace tab for the top tab strip. */
export function openTabsBarItems(attention: TabAttention[]): TabAttention[] {
  return attention;
}
