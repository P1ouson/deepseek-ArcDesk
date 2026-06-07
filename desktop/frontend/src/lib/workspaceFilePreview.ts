function normalizePath(path: string): string {
  return path.replace(/\\/g, "/");
}

export function basename(path: string): string {
  const parts = normalizePath(path).split("/").filter(Boolean);
  return parts[parts.length - 1] ?? path;
}

export function shortCwd(cwd?: string): string {
  if (!cwd) return "";
  const parts = normalizePath(cwd).split("/").filter(Boolean);
  if (parts.length <= 2) return cwd;
  return `…/${parts.slice(-2).join("/")}`;
}

export function parentPath(path: string): string {
  const clean = normalizePath(path).replace(/\/$/, "");
  const parts = clean.split("/").filter(Boolean);
  return parts.slice(0, -1).join("/");
}

export function parentDirs(path: string): string[] {
  const parts = normalizePath(path).split("/").filter(Boolean);
  const dirs: string[] = [""];
  let acc = "";
  for (let i = 0; i < parts.length - 1; i++) {
    acc += `${parts[i]}/`;
    dirs.push(acc);
  }
  return dirs;
}

export function languageFor(path: string): string | undefined {
  const name = basename(path).toLowerCase();
  const ext = name.includes(".") ? name.slice(name.lastIndexOf(".") + 1) : name;
  const byExt: Record<string, string> = {
    css: "css",
    go: "go",
    html: "html",
    js: "javascript",
    json: "json",
    jsx: "jsx",
    md: "markdown",
    py: "python",
    rs: "rust",
    sh: "bash",
    toml: "toml",
    ts: "typescript",
    tsx: "tsx",
    yaml: "yaml",
    yml: "yaml",
  };
  return byExt[ext];
}

export function isImagePath(path: string): boolean {
  const ext = basename(path).toLowerCase();
  const dot = ext.lastIndexOf(".");
  if (dot < 0) return false;
  return [".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico"].includes(ext.slice(dot));
}

export function isSvgPath(path: string): boolean {
  return basename(path).toLowerCase().endsWith(".svg");
}

export function fenceFor(text: string): string {
  let longest = 0;
  for (const match of text.matchAll(/`+/g)) {
    longest = Math.max(longest, match[0].length);
  }
  return "`".repeat(Math.max(3, longest + 1));
}

export function formatSelectionReference(path: string, text: string): string {
  const body = text.replace(/\r\n|\r/g, "\n").trimEnd();
  const fence = fenceFor(body);
  const lang = languageFor(path);
  return `From \`${path}\`:\n\n${fence}${lang ?? ""}\n${body}\n${fence}`;
}
