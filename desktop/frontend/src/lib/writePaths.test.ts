import { describe, expect, it } from "vitest";
import { isDirectChild, isPathUnderRoot, normalizeWritePath, parentWritePath, pathsEqual } from "./writePaths";

describe("writePaths", () => {
  it("normalizes separators", () => {
    expect(normalizeWritePath("E:\\foo\\bar\\")).toBe("E:/foo/bar");
  });

  it("compares Windows paths case-insensitively", () => {
    expect(pathsEqual("E:/Project/docs", "e:/project/docs")).toBe(true);
    expect(isDirectChild("E:/Project/docs/readme.md", "e:/project/docs")).toBe(true);
  });

  it("detects path roots and parents", () => {
    expect(isPathUnderRoot("E:/Project/docs/readme.md", "E:/Project")).toBe(true);
    expect(parentWritePath("E:/Project/docs/readme.md")).toBe("E:/Project/docs");
  });
});
