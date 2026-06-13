import { EventsOn } from "../../wailsjs/runtime/runtime";

export function onAgentEvent(cb: () => void) {
  EventsOn("agent:event", cb);
}

const CHANNEL = "agent:ready";
export function onReady(cb: () => void) {
  EventsOn(CHANNEL, cb);
}
