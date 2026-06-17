import { describe, expect, it } from "vitest";
import { getWritePreviewKind, isMarkdownDocumentPath, isWordDocumentPath } from "./writeDocument";

describe("writeDocument", () => {
  it("detects Word documents", () => {
    expect(isWordDocumentPath("draft.docx")).toBe(true);
    expect(isWordDocumentPath("legacy.doc")).toBe(true);
    expect(isWordDocumentPath("notes.txt")).toBe(false);
  });

  it("detects Markdown documents", () => {
    expect(isMarkdownDocumentPath("readme.md")).toBe(true);
    expect(isMarkdownDocumentPath("post.markdown")).toBe(true);
    expect(isMarkdownDocumentPath("page.mdx")).toBe(true);
    expect(isMarkdownDocumentPath("notes.txt")).toBe(false);
  });

  it("classifies preview kinds", () => {
    expect(getWritePreviewKind("a.docx")).toBe("word");
    expect(getWritePreviewKind("a.md")).toBe("markdown");
    expect(getWritePreviewKind("requirements.txt")).toBe("plain");
    expect(getWritePreviewKind("notes.rst")).toBe("plain");
  });
});
