import { describe, expect, it } from "vitest";
import { subjectOf } from "./tools";

describe("subjectOf", () => {
  it("extracts bash command", () => {
    expect(subjectOf("bash", JSON.stringify({ command: "ls -la" }))).toBe("ls -la");
  });

  it("extracts grep pattern", () => {
    expect(subjectOf("grep", JSON.stringify({ pattern: "TODO", path: "src" }))).toBe("TODO");
  });

  it("returns empty for invalid json args", () => {
    expect(subjectOf("read_file", "{")).toBe("");
  });

  it("prefers file path for generic tools", () => {
    expect(subjectOf("read_file", JSON.stringify({ path: "README.md" }))).toBe("README.md");
  });
});
