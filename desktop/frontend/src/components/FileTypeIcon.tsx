import type { LucideIcon } from "lucide-react";
import {
  Braces,
  File,
  FileCode,
  FileImage,
  FileJson,
  FileText,
  FileType,
  Folder,
  FolderOpen,
  Terminal,
} from "lucide-react";

export type FileIconTone =
  | "folder"
  | "folder-open"
  | "typescript"
  | "javascript"
  | "json"
  | "markdown"
  | "image"
  | "css"
  | "html"
  | "python"
  | "go"
  | "rust"
  | "shell"
  | "yaml"
  | "word"
  | "generic";

function fileIconTone(name: string, isDir: boolean, isOpen?: boolean): FileIconTone {
  if (isDir) return isOpen ? "folder-open" : "folder";
  const lower = name.toLowerCase();
  const dot = lower.lastIndexOf(".");
  const ext = dot >= 0 ? lower.slice(dot + 1) : "";
  const base = dot >= 0 ? lower.slice(0, dot) : lower;

  if (["ts", "tsx", "mts", "cts"].includes(ext)) return "typescript";
  if (["js", "jsx", "mjs", "cjs"].includes(ext)) return "javascript";
  if (ext === "json" || ext === "jsonc") return "json";
  if (["md", "mdx", "txt", "rst"].includes(ext)) return "markdown";
  if (["doc", "docx"].includes(ext)) return "word";
  if (["png", "jpg", "jpeg", "gif", "webp", "bmp", "ico", "svg"].includes(ext)) return "image";
  if (["css", "scss", "sass", "less"].includes(ext)) return "css";
  if (["html", "htm", "vue", "svelte"].includes(ext)) return "html";
  if (ext === "py" || ext === "pyw") return "python";
  if (ext === "go") return "go";
  if (ext === "rs") return "rust";
  if (["sh", "bash", "zsh", "fish"].includes(ext) || base === "dockerfile") return "shell";
  if (["yaml", "yml", "toml"].includes(ext)) return "yaml";
  return "generic";
}

const ICONS: Record<FileIconTone, LucideIcon> = {
  folder: Folder,
  "folder-open": FolderOpen,
  typescript: FileCode,
  javascript: FileCode,
  json: FileJson,
  markdown: FileText,
  image: FileImage,
  css: Braces,
  html: FileType,
  python: FileCode,
  go: FileCode,
  rust: FileCode,
  shell: Terminal,
  yaml: Braces,
  word: FileType,
  generic: File,
};

export function FileTypeIcon({
  name,
  isDir,
  isOpen,
  className,
}: {
  name: string;
  isDir: boolean;
  isOpen?: boolean;
  className?: string;
}) {
  const tone = fileIconTone(name, isDir, isOpen);
  const Icon = ICONS[tone];
  return (
    <Icon
      size={15}
      strokeWidth={1.75}
      className={`files-panel__ico files-panel__ico--${tone}${className ? ` ${className}` : ""}`}
      aria-hidden="true"
    />
  );
}
