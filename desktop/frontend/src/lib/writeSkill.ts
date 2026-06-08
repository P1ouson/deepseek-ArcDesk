/** Global skill auto-invoked in ARCDESK write mode (marketingskills/copywriting). */
export const WRITE_COPY_SKILL = "copywriting";

const WRITE_SKILL_SLASH_RE = /^\/(copywriting|write)\b/i;

/** Prefix agent submit text with /copywriting so the skill body loads for write-mode turns. */
export function applyWriteModeSkill(displayText: string, submitText: string): { displayText: string; submitText: string } {
  const body = submitText.trim();
  if (!body || body.startsWith("!") || WRITE_SKILL_SLASH_RE.test(body)) {
    return { displayText, submitText: body };
  }
  return {
    displayText,
    submitText: `/${WRITE_COPY_SKILL} ${body}`,
  };
}
