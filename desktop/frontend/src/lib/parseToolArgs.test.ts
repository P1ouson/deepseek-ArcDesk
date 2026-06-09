import { describe, expect, it } from "vitest";
import { parseToolArgs, toolArgString } from "./parseToolArgs";

describe("parseToolArgs", () => {
  it("parses json tool args", () => {
    expect(parseToolArgs('{"path":"a.ts"}')).toEqual({ path: "a.ts" });
  });

  it("returns empty object for invalid json", () => {
    expect(parseToolArgs("{")).toEqual({});
  });
});

describe("toolArgString", () => {
  it("reads string fields", () => {
    expect(toolArgString({ command: "ls" }, "command")).toBe("ls");
  });

  it("returns empty for missing or non-string fields", () => {
    expect(toolArgString({}, "command")).toBe("");
    expect(toolArgString({ command: 1 }, "command")).toBe("");
  });
});
