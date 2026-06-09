import type { WireApproval, WireAsk } from "./types";

let lastKey = "";

function notifyKey(kind: "approval" | "ask", id: string) {
  return `${kind}:${id}`;
}

export function notifyAgentDecision(
  approval: WireApproval | null | undefined,
  ask: WireAsk | null | undefined,
  labels: { approvalTitle: string; askTitle: string; bodyApproval: (tool: string) => string; bodyAsk: string },
) {
  if (typeof Notification === "undefined") return;
  if (Notification.permission === "default") {
    void Notification.requestPermission();
  }
  if (Notification.permission !== "granted") return;

  if (approval) {
    const key = notifyKey("approval", approval.id);
    if (key === lastKey) return;
    lastKey = key;
    const body =
      approval.tool === "exit_plan_mode"
        ? labels.approvalTitle
        : labels.bodyApproval(approval.tool) + (approval.subject ? `\n${approval.subject.split("\n")[0]}` : "");
    new Notification(labels.approvalTitle, { body, tag: key });
    return;
  }
  if (ask) {
    const key = notifyKey("ask", ask.id);
    if (key === lastKey) return;
    lastKey = key;
    const q = ask.questions[0]?.prompt ?? "";
    new Notification(labels.askTitle, { body: q || labels.bodyAsk, tag: key });
    return;
  }
  lastKey = "";
}

export function clearAgentDecisionNotifications() {
  lastKey = "";
}
