import { describe, expect, it } from "vitest";
import { humanizeUserError, toErrorMessage } from "./errors";

describe("humanizeUserError", () => {
  it("maps provider-prefixed context canceled errors", () => {
    expect(
      humanizeUserError(
        'deepseek-flash: request failed: Post "https://zenmux.ai/api/v1/chat/completions": context canceled',
      ),
    ).toBe("Request canceled.");
  });

  it("maps validate-wrapped fetch failures", () => {
    expect(
      humanizeUserError("validate: fetch models: request failed: Get \"https://x/v1/models\": EOF"),
    ).toBe("Could not reach the API. Check the base URL, network, and proxy settings.");
  });

  it("maps provider status strings", () => {
    expect(humanizeUserError("deepseek-flash: status 402: Insufficient Balance")).toContain("402");
  });

  it("keeps plain provider API messages without transport noise", () => {
    expect(humanizeUserError("deepseek-flash: model is overloaded, please retry later")).toBe(
      "model is overloaded, please retry later",
    );
  });

  it("passes through unrelated errors", () => {
    expect(humanizeUserError("no active tab")).toBe("no active tab");
  });
});

describe("toErrorMessage", () => {
  it("humanizes Error instances", () => {
    expect(toErrorMessage(new Error("deepseek-flash: read stream: unexpected EOF"))).toBe(
      "Connection to the model was interrupted. Please retry.",
    );
  });
});
