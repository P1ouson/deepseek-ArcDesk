import { describe, expect, it } from "vitest";
import { prettyJson } from "./prettyJson";

describe("prettyJson", () => {
  it("formats valid JSON", () => {
    expect(prettyJson('{"a":1}')).toBe('{\n  "a": 1\n}');
  });

  it("returns raw text for invalid JSON", () => {
    expect(prettyJson("{not json")).toBe("{not json");
  });
});
