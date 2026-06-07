import type { EditorProps } from "../CodeViewer";
import { highlightToHtml } from "../../lib/highlight";

// HljsCode is the syntax-highlighted default behind the code editor seam. It
// renders highlight.js token markup into a <pre>; token colors live in styles.css
// (.hljs-*). To upgrade to a full editor, point CodeViewer.tsx's lazy import at a
// Monaco/CodeMirror module honoring the same EditorProps.
export default function HljsCode({ value, language, maxHeight, lineNumbers }: EditorProps) {
  const html = highlightToHtml(value, language);
  const heightStyle = maxHeight ? { maxHeight } : undefined;

  if (lineNumbers) {
    const lines = value.replace(/\r\n|\r/g, "\n").split("\n");
    return (
      <div className="code code--with-lines" data-lang={language} style={heightStyle}>
        <div className="code__gutter" aria-hidden="true">
          {lines.map((_, index) => (
            <span key={index}>{index + 1}</span>
          ))}
        </div>
        <pre className="code__content hljs">
          <code dangerouslySetInnerHTML={{ __html: html }} />
        </pre>
      </div>
    );
  }

  return (
    <pre className="code hljs" data-lang={language} style={heightStyle}>
      <code dangerouslySetInnerHTML={{ __html: html }} />
    </pre>
  );
}
