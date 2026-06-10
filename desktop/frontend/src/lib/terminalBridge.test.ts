import { afterEach, describe, expect, it, vi } from "vitest";
import { decodeTerminalPayload, resetTerminalDecodeWarnForTests } from "./terminalBridge";

describe("decodeTerminalPayload", () => {
  afterEach(() => {
    resetTerminalDecodeWarnForTests();
    vi.restoreAllMocks();
  });

  it("decodes valid base64 payload", () => {
    const text = "hello terminal";
    const encoded = btoa(text);
    const out = decodeTerminalPayload(encoded);
    expect(out).not.toBeNull();
    expect(new TextDecoder().decode(out!)).toBe(text);
  });

  it("does not throw on malformed base64", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    expect(() => decodeTerminalPayload("not!!!valid")).not.toThrow();
    expect(decodeTerminalPayload("not!!!valid")).toBeNull();
    expect(warn).toHaveBeenCalledTimes(1);
  });

  it("recovers after malformed payload with valid payload", () => {
    vi.spyOn(console, "warn").mockImplementation(() => {});
    expect(decodeTerminalPayload("%%%")).toBeNull();
    const ok = decodeTerminalPayload(btoa("next"));
    expect(ok).not.toBeNull();
    expect(new TextDecoder().decode(ok!)).toBe("next");
  });
});
