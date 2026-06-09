import { describe, expect, it } from "vitest";
import {
  CODE_REVIEW_SETTINGS_EVENT,
  DESKTOP_GIT_SETTINGS_EVENT,
  TAB_METAS_CHANGED_EVENT,
} from "./events";

describe("events", () => {
  it("uses consistent ARCDESK: prefix for settings buses", () => {
    expect(DESKTOP_GIT_SETTINGS_EVENT).toBe("ARCDESK:desktop-git-settings");
    expect(CODE_REVIEW_SETTINGS_EVENT).toBe("ARCDESK:code-review-settings");
    expect(TAB_METAS_CHANGED_EVENT).toBe("ARCDESK:tab-metas-changed");
  });
});
