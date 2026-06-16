/** Global skill available in ARCDESK write mode (marketingskills/copywriting). */
export const WRITE_COPY_SKILL = "copywriting";

const WRITE_SKILL_SLASH_RE = /^\/(copywriting|write)\b/i;

/** Marketing / copy-editing intent — not code exploration or project work. */
const COPYWRITING_INTENT_RE =
  /(文案|营销|landing|homepage|headline|cta|tagline|广告语|软文|转化|copywriting|marketing copy|value prop|hero section|above the fold|make this more compelling|rewrite this page|improve this copy)/i;

/** Task touches code, repos, builds, or technical project facts. */
const CODE_LINKAGE_INTENT_RE =
  /(\.(py|go|ts|tsx|js|jsx|cs|java|rs|cpp|c|h|md|toml|yaml|yml|json)\b|read_file|grep|glob|ls\b|代码|项目|训练|模型|函数|模块|bug|fix|refactor|implement|api|git|test|debug|compile|build|课程设计|说明书|实验|数据集|样本|架构|仓库|repo|workspace|\.venv|requirements\.txt)/i;

export type WriteSkillContext = {
  /** User has a linked code project workspace for this write turn. */
  hasLinkedCodeProject?: boolean;
  /** A document is open in the write editor. */
  hasOpenDocument?: boolean;
};

/** Whether to inline the full copywriting skill body for this write-mode turn. */
export function shouldInlineCopywritingSkill(submitText: string, ctx: WriteSkillContext = {}): boolean {
  const body = submitText.trim();
  if (!body || body.startsWith("!") || WRITE_SKILL_SLASH_RE.test(body)) {
    return false;
  }
  const codeLinked = Boolean(ctx.hasLinkedCodeProject);
  const codeTask = codeLinked && CODE_LINKAGE_INTENT_RE.test(body);
  if (codeTask) {
    return false;
  }
  if (COPYWRITING_INTENT_RE.test(body)) {
    return true;
  }
  // Document-only rewrite with no code project — still a writing task.
  if (ctx.hasOpenDocument && !codeLinked) {
    return true;
  }
  return false;
}

/** Prefix agent submit text with /copywriting only when the turn is a writing task. */
export function applyWriteModeSkill(
  displayText: string,
  submitText: string,
  ctx: WriteSkillContext = {},
): { displayText: string; submitText: string } {
  const body = submitText.trim();
  if (!shouldInlineCopywritingSkill(body, ctx)) {
    return { displayText, submitText: body };
  }
  return {
    displayText,
    submitText: `/${WRITE_COPY_SKILL} ${body}`,
  };
}
