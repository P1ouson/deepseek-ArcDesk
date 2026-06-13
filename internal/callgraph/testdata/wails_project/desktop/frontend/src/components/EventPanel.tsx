import { EventsOn } from "../../wailsjs/runtime/runtime";

export function EventPanel() {
  EventsOn("agent:event", () => {
    console.log("event");
  });
}
