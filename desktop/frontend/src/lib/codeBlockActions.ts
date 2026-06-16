import { app, openExternal } from "./bridge";

const EXT: Record<string, string> = {
  html: "html",
  htm: "html",
  javascript: "js",
  js: "js",
  typescript: "ts",
  ts: "ts",
  python: "py",
  py: "py",
  bash: "sh",
  sh: "sh",
  shell: "sh",
  zsh: "sh",
  powershell: "ps1",
  pwsh: "ps1",
  markdown: "md",
  md: "md",
  json: "json",
  css: "css",
  sql: "sql",
  go: "go",
  rust: "rs",
  java: "java",
  kotlin: "kt",
  swift: "swift",
  yaml: "yml",
  yml: "yml",
  xml: "xml",
  svg: "svg",
  plaintext: "txt",
  text: "txt",
};

const SHELL_LANGS = new Set(["bash", "sh", "shell", "zsh", "powershell", "pwsh", "cmd", "bat"]);
const PREVIEW_LANGS = new Set(["html", "htm", "svg"]);

export function codeBlockLanguageLabel(language?: string): string {
  const lang = (language ?? "").trim().toLowerCase();
  if (!lang || lang === "plaintext" || lang === "text") return "text";
  return lang;
}

export function codeBlockFilename(language: string | undefined): string {
  const ext = EXT[(language ?? "").trim().toLowerCase()] ?? "txt";
  const stamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
  return `snippet-${stamp}.${ext}`;
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

export function downloadCodeBlock(value: string, language?: string) {
  const filename = codeBlockFilename(language);
  const mime = filename.endsWith(".html") ? "text/html;charset=utf-8" : "text/plain;charset=utf-8";
  downloadBlob(new Blob([value], { type: mime }), filename);
}

export function canRunCodeBlock(language?: string): boolean {
  const lang = (language ?? "").trim().toLowerCase();
  if (!lang || lang === "plaintext" || lang === "text") return false;
  return PREVIEW_LANGS.has(lang) || SHELL_LANGS.has(lang);
}

export function runCodeBlock(value: string, language?: string) {
  const lang = (language ?? "").trim().toLowerCase();
  const body = value.trim();
  if (!body) return;

  if (PREVIEW_LANGS.has(lang)) {
    const blob = new Blob([body], { type: "text/html;charset=utf-8" });
    openExternal(URL.createObjectURL(blob));
    return;
  }

  if (SHELL_LANGS.has(lang)) {
    void app.RunShell(body);
  }
}
