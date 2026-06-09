import { describe, expect, it } from "vitest";
import { hasGitChange, isDeletedGitChange } from "./workspaceChangeHelpers";
import type { WorkspaceChangeView } from "./types";

function row(partial: Partial<WorkspaceChangeView>): WorkspaceChangeView {
  return {
    path: "a.ts",
    sources: [],
    ...partial,
  };
}

describe("workspaceChangeHelpers", () => {
  it("detects git rows", () => {
    expect(hasGitChange(row({ sources: ["git"] }))).toBe(true);
    expect(hasGitChange(row({ sources: ["session"] }))).toBe(false);
  });

  it("detects deleted git status", () => {
    expect(isDeletedGitChange(row({ gitStatus: " D" }))).toBe(true);
    expect(isDeletedGitChange(row({ gitStatus: "M" }))).toBe(false);
  });
});
