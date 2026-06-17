import { describe, expect, it } from "vitest";
import {
  activeCodeWorkspaceRoot,
  planCodeWorkspaceReconcile,
} from "./composerWorkspace";
import type { TabMeta } from "./types";

function tab(partial: Partial<TabMeta>): TabMeta {
  return {
    id: "t1",
    scope: "global",
    workspaceRoot: "",
    workspaceName: "",
    topicId: "",
    topicTitle: "",
    label: "",
    ready: true,
    running: false,
    mode: "normal",
    active: true,
    cwd: "",
    ...partial,
  };
}

describe("activeCodeWorkspaceRoot", () => {
  it("returns project tab workspace root", () => {
    expect(activeCodeWorkspaceRoot(tab({ scope: "project", workspaceRoot: "E:\\proj" }))).toBe("E:\\proj");
  });

  it("ignores global tabs and empty roots", () => {
    expect(activeCodeWorkspaceRoot(tab({ scope: "global" }))).toBeUndefined();
    expect(activeCodeWorkspaceRoot(tab({ scope: "project", workspaceRoot: "" }))).toBeUndefined();
    expect(activeCodeWorkspaceRoot(undefined)).toBeUndefined();
  });
});

describe("planCodeWorkspaceReconcile", () => {
  it("syncs stored root to the active project tab", () => {
    const active = tab({ scope: "project", workspaceRoot: "E:\\active" });
    expect(planCodeWorkspaceReconcile(active, [active], "E:\\stale")).toEqual({
      kind: "syncToActive",
      path: "E:\\active",
    });
  });

  it("clears orphan stored root when no matching project tab is open", () => {
    const active = tab({ scope: "global" });
    expect(planCodeWorkspaceReconcile(active, [active], "E:\\stale")).toEqual({ kind: "clearOrphan" });
  });

  it("keeps stored root when another open tab still owns it", () => {
    const active = tab({ scope: "global" });
    const other = tab({ id: "t2", scope: "project", workspaceRoot: "E:\\stored" });
    expect(planCodeWorkspaceReconcile(active, [active, other], "E:\\stored")).toEqual({ kind: "noop" });
  });
});
