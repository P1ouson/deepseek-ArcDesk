import { describe, expect, it } from "vitest";
import { isPathUnderWorkspace } from "./workspaceRemoval";

describe("isPathUnderWorkspace", () => {
  it("matches files inside a workspace root", () => {
    expect(isPathUnderWorkspace("E:\\proj\\src\\main.go", "E:\\proj")).toBe(true);
    expect(isPathUnderWorkspace("E:/proj", "E:\\proj\\")).toBe(true);
  });

  it("rejects paths outside the workspace root", () => {
    expect(isPathUnderWorkspace("E:\\other\\main.go", "E:\\proj")).toBe(false);
    expect(isPathUnderWorkspace("", "E:\\proj")).toBe(false);
  });
});
