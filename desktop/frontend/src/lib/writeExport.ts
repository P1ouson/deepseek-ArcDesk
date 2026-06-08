function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function inlineMarkdown(text: string): string {
  return escapeHtml(text)
    .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
    .replace(/\*(.+?)\*/g, "<em>$1</em>")
    .replace(/`([^`]+)`/g, "<code>$1</code>")
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>');
}

export function markdownToHtmlDocument(title: string, markdown: string): string {
  const lines = markdown.split(/\r?\n/);
  const blocks: string[] = [];
  let inCode = false;
  let codeLang = "";
  let codeLines: string[] = [];
  let listType: "ul" | "ol" | null = null;
  let listItems: string[] = [];

  const flushList = () => {
    if (!listType || listItems.length === 0) return;
    const tag = listType;
    blocks.push(`<${tag}>${listItems.map((item) => `<li>${inlineMarkdown(item)}</li>`).join("")}</${tag}>`);
    listType = null;
    listItems = [];
  };

  for (const line of lines) {
    if (line.startsWith("```")) {
      if (inCode) {
        blocks.push(`<pre><code class="language-${escapeHtml(codeLang)}">${escapeHtml(codeLines.join("\n"))}</code></pre>`);
        inCode = false;
        codeLang = "";
        codeLines = [];
      } else {
        flushList();
        inCode = true;
        codeLang = line.slice(3).trim();
      }
      continue;
    }
    if (inCode) {
      codeLines.push(line);
      continue;
    }

    const heading = /^(#{1,6})\s+(.+)$/.exec(line);
    if (heading) {
      flushList();
      const level = heading[1]!.length;
      blocks.push(`<h${level}>${inlineMarkdown(heading[2]!)}</h${level}>`);
      continue;
    }

    const ul = /^[-*+]\s+(.+)$/.exec(line);
    if (ul) {
      if (listType !== "ul") {
        flushList();
        listType = "ul";
      }
      listItems.push(ul[1]!);
      continue;
    }

    const ol = /^\d+\.\s+(.+)$/.exec(line);
    if (ol) {
      if (listType !== "ol") {
        flushList();
        listType = "ol";
      }
      listItems.push(ol[1]!);
      continue;
    }

    if (!line.trim()) {
      flushList();
      continue;
    }

    flushList();
    blocks.push(`<p>${inlineMarkdown(line)}</p>`);
  }

  flushList();
  if (inCode && codeLines.length) {
    blocks.push(`<pre><code>${escapeHtml(codeLines.join("\n"))}</code></pre>`);
  }

  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>${escapeHtml(title)}</title>
  <style>
    body { font-family: Georgia, "Times New Roman", serif; line-height: 1.65; max-width: 720px; margin: 48px auto; padding: 0 24px; color: #1a1a1a; }
    h1,h2,h3,h4,h5,h6 { line-height: 1.25; margin-top: 1.4em; }
    pre { background: #f5f5f5; padding: 12px 14px; overflow: auto; border-radius: 8px; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.92em; }
    a { color: #4854b8; }
  </style>
</head>
<body>
${blocks.join("\n")}
</body>
</html>`;
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

export function exportMarkdownFile(content: string, filename: string) {
  downloadBlob(new Blob([content], { type: "text/markdown;charset=utf-8" }), filename.endsWith(".md") ? filename : `${filename}.md`);
}

export function exportHtmlFile(content: string, filename: string) {
  const title = filename.replace(/\.[^.]+$/, "") || "draft";
  const html = markdownToHtmlDocument(title, content);
  downloadBlob(new Blob([html], { type: "text/html;charset=utf-8" }), filename.endsWith(".html") ? filename : `${filename.replace(/\.[^.]+$/, "")}.html`);
}

export function exportDocxFile(content: string, filename: string) {
  const title = filename.replace(/\.[^.]+$/, "") || "draft";
  const html = markdownToHtmlDocument(title, content);
  const wrapped = `<html xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:w="urn:schemas-microsoft-com:office:word"><head><meta charset="utf-8"></head><body>${html.match(/<body>([\s\S]*)<\/body>/i)?.[1] ?? html}</body></html>`;
  downloadBlob(new Blob(["\ufeff", wrapped], { type: "application/msword" }), filename.endsWith(".docx") ? filename : `${filename.replace(/\.[^.]+$/, "")}.docx`);
}

export function exportPdfFile(content: string, filename: string) {
  const title = filename.replace(/\.[^.]+$/, "") || "draft";
  const html = markdownToHtmlDocument(title, content);
  const frame = document.createElement("iframe");
  frame.style.position = "fixed";
  frame.style.right = "0";
  frame.style.bottom = "0";
  frame.style.width = "0";
  frame.style.height = "0";
  frame.style.border = "0";
  document.body.appendChild(frame);
  const doc = frame.contentDocument;
  if (!doc) {
    frame.remove();
    return;
  }
  doc.open();
  doc.write(html);
  doc.close();
  frame.contentWindow?.focus();
  frame.contentWindow?.print();
  window.setTimeout(() => frame.remove(), 1000);
}
