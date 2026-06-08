export const CODE_HIGHLIGHT_THEMES = ["editorial", "github", "monokai", "solarized"] as const;

export type CodeHighlightTheme = (typeof CODE_HIGHLIGHT_THEMES)[number];

const STORAGE_KEY = "ARCDESK.codeHighlightTheme";

export function isCodeHighlightTheme(value: unknown): value is CodeHighlightTheme {
  return typeof value === "string" && (CODE_HIGHLIGHT_THEMES as readonly string[]).includes(value);
}

export function getCodeHighlightTheme(): CodeHighlightTheme {
  if (typeof window === "undefined") return "editorial";
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return isCodeHighlightTheme(stored) ? stored : "editorial";
  } catch {
    return "editorial";
  }
}

export function applyCodeHighlightTheme(theme: CodeHighlightTheme): void {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  if (theme === "editorial") root.removeAttribute("data-hl-theme");
  else root.setAttribute("data-hl-theme", theme);
  try {
    window.localStorage.setItem(STORAGE_KEY, theme);
  } catch {
    /* ignore */
  }
}

export function initCodeHighlightTheme(): void {
  applyCodeHighlightTheme(getCodeHighlightTheme());
}
