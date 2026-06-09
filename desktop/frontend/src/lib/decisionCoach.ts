export type DecisionCoachTopic = "plan" | "ask" | "yolo" | "approval";

const KEY = "arcdesk.decisionCoach.v1";

function read(): Record<DecisionCoachTopic, boolean> {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return { plan: false, ask: false, yolo: false, approval: false };
    const parsed = JSON.parse(raw) as Partial<Record<DecisionCoachTopic, boolean>>;
    return {
      plan: parsed.plan === true,
      ask: parsed.ask === true,
      yolo: parsed.yolo === true,
      approval: parsed.approval === true,
    };
  } catch {
    return { plan: false, ask: false, yolo: false, approval: false };
  }
}

function write(state: Record<DecisionCoachTopic, boolean>) {
  localStorage.setItem(KEY, JSON.stringify(state));
}

export function shouldShowDecisionCoach(topic: DecisionCoachTopic): boolean {
  return !read()[topic];
}

export function markDecisionCoachSeen(topic: DecisionCoachTopic) {
  const next = read();
  next[topic] = true;
  write(next);
}
