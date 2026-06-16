import { describe, expect, it } from "vitest";
import { applyWriteModeSkill, shouldInlineCopywritingSkill } from "./writeSkill";

describe("shouldInlineCopywritingSkill", () => {
  it("skips code-linked project tasks", () => {
    expect(
      shouldInlineCopywritingSkill("帮我看看训练模型的时间写在哪里", { hasLinkedCodeProject: true }),
    ).toBe(false);
  });

  it("allows marketing copy tasks", () => {
    expect(shouldInlineCopywritingSkill("帮我改写首页 hero 文案", {})).toBe(true);
  });

  it("allows document rewrite when no code project is linked", () => {
    expect(shouldInlineCopywritingSkill("润色这段引言", { hasOpenDocument: true })).toBe(true);
  });

  it("skips explicit slash commands", () => {
    expect(shouldInlineCopywritingSkill("/explore map the repo", {})).toBe(false);
  });
});

describe("applyWriteModeSkill", () => {
  it("does not prefix code project questions", () => {
    const out = applyWriteModeSkill("用户问题", "课程设计报告里训练时间写在哪", {
      hasLinkedCodeProject: true,
    });
    expect(out.submitText).toBe("课程设计报告里训练时间写在哪");
    expect(out.submitText).not.toMatch(/^\/copywriting/);
  });

  it("prefixes copywriting tasks", () => {
    const out = applyWriteModeSkill("改文案", "rewrite the landing page headline", {});
    expect(out.submitText).toMatch(/^\/copywriting /);
  });
});
