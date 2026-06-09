/** Suggest a permission rule string for "always allow" based on tool + subject. */
export function suggestPermissionRule(tool: string, subject: string): string | null {
  const name = tool.trim();
  if (!name || name === "exit_plan_mode") return null;

  const firstLine = subject.trim().split("\n").find((line) => line.trim())?.trim() ?? "";
  if (!firstLine) return name;

  if (name === "bash" || name === "run_shell") {
    const token = firstLine.split(/\s+/)[0]?.replace(/^["']|["']$/g, "") ?? "";
    if (token.length >= 2) return `bash(${token}*)`;
    return "bash(*)";
  }

  if (firstLine.length <= 96 && !firstLine.includes("\n")) {
    const literal = firstLine.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
    return `${name}(${literal})`;
  }

  return name;
}
