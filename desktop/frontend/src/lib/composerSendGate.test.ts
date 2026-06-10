import { describe, expect, it } from "vitest";
import { isComposerSendDisabled } from "./composerSendGate";

describe("isComposerSendDisabled", () => {
  it("blocks send when startupErr is set", () => {
    expect(isComposerSendDisabled({ ready: true, startupErr: "boot failed" }, null, null)).toBe(true);
  });

  it("allows send when ready and no startupErr", () => {
    expect(isComposerSendDisabled({ ready: true }, null, null)).toBe(false);
  });

  it("blocks send while controller is not ready", () => {
    expect(isComposerSendDisabled({ ready: false }, null, null)).toBe(true);
  });

  it("blocks send while Wails runtime is not ready", () => {
    expect(isComposerSendDisabled({ ready: true }, null, null, false)).toBe(true);
  });

  it("allows send when ready and runtime is ready", () => {
    expect(isComposerSendDisabled({ ready: true }, null, null, true)).toBe(false);
  });
});
