// theme.ts — System / Light / Dark appearance for ArcDesk Desktop.
// Applies data-theme + data-theme-style on <html> and syncs native Wails window chrome.

import {
  WindowSetDarkTheme,
  WindowSetLightTheme,
  WindowSetSystemDefaultTheme,
  WindowSetBackgroundColour,
} from "../../wailsjs/runtime/runtime";
import { applyAppearancePrefs, syncSurfacePresetsToTheme } from "./appearancePrefs";

export type Theme = "auto" | "light" | "dark";
export type ResolvedTheme = Exclude<Theme, "auto">;

export const THEME_STYLES = [
  "graphite",
  "ember",
  "aurora",
  "midnight",
  "cobalt",
  "sandstone",
  "porcelain",
  "linen",
  "glacier",
] as const;

export type ThemeStyle = (typeof THEME_STYLES)[number];

const DEFAULT_THEME: Theme = "light";
const DEFAULT_DARK_STYLE: ThemeStyle = "graphite";
const DEFAULT_LIGHT_STYLE: ThemeStyle = "glacier";
const THEME_KEY = "arcdesk-theme";
const STYLE_KEY = "arcdesk-theme-style";
let currentTheme: Theme = DEFAULT_THEME;
let currentStyle: ThemeStyle = DEFAULT_LIGHT_STYLE;
let systemThemeListenerInstalled = false;

const DARK_STYLES = new Set<ThemeStyle>(["graphite", "ember", "aurora", "midnight", "cobalt"]);
const LIGHT_STYLES = new Set<ThemeStyle>(["sandstone", "porcelain", "linen", "glacier"]);

export function isThemeStyle(value: unknown): value is ThemeStyle {
  return typeof value === "string" && (THEME_STYLES as readonly string[]).includes(value);
}

function resolveTheme(theme: Theme): ResolvedTheme {
  if (theme === "light" || theme === "dark") return theme;
  if (typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: light)").matches) return "light";
  return "dark";
}

function defaultStyleForResolved(resolved: ResolvedTheme): ThemeStyle {
  return resolved === "light" ? DEFAULT_LIGHT_STYLE : DEFAULT_DARK_STYLE;
}

function syncSystemTheme(theme: Theme): void {
  if (typeof document === "undefined") return;
  if (theme !== "auto") return;
  applyTheme("auto", getThemeStyle(), { syncSurfaces: true });
}

function installSystemThemeListener(): void {
  if (systemThemeListenerInstalled || typeof window === "undefined" || !window.matchMedia) return;
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const onChange = () => syncSystemTheme(getTheme());
  if (typeof media.addEventListener === "function") media.addEventListener("change", onChange);
  else if (typeof media.addListener === "function") media.addListener(onChange);
  systemThemeListenerInstalled = true;
}

export function normalizeThemePreference(value: unknown): Theme {
  if (typeof value === "object" && value !== null) {
    return normalizeThemePreference((value as { mode?: unknown }).mode);
  }
  if (typeof value !== "string") return DEFAULT_THEME;
  switch (value) {
    case "auto":
      return "auto";
    case "light":
    case "focus":
    case "forest":
      return "light";
    case "dark":
    case "midnight":
    case "contrast":
      return "dark";
    default:
      return DEFAULT_THEME;
  }
}

export function getTheme(): Theme {
  return currentTheme;
}

export function getResolvedTheme(theme: Theme = getTheme()): ResolvedTheme {
  return resolveTheme(theme);
}

export function defaultStyleForTheme(theme: Theme): ThemeStyle {
  return defaultStyleForResolved(resolveTheme(theme));
}

export function themeForStyle(style: ThemeStyle): ResolvedTheme {
  if (LIGHT_STYLES.has(style)) return "light";
  if (DARK_STYLES.has(style)) return "dark";
  return getResolvedTheme();
}

export function getThemeStyle(theme: Theme = getTheme()): ThemeStyle {
  const resolved = resolveTheme(theme);
  if (resolved === "light" && LIGHT_STYLES.has(currentStyle)) return currentStyle;
  if (resolved === "dark" && DARK_STYLES.has(currentStyle)) return currentStyle;
  return defaultStyleForResolved(resolved);
}

export function normalizeThemeStyleForTheme(style: string | undefined, theme: Theme): ThemeStyle {
  const normalized = typeof style === "string" ? style.trim().toLowerCase() : "";
  if (isThemeStyle(normalized)) {
    const resolved = resolveTheme(theme);
    if (resolved === "light" && LIGHT_STYLES.has(normalized)) return normalized;
    if (resolved === "dark" && DARK_STYLES.has(normalized)) return normalized;
  }
  return defaultStyleForTheme(theme);
}

export function stylesForTheme(theme: Theme): ThemeStyle[] {
  return resolveTheme(theme) === "light"
    ? ["glacier", "sandstone", "porcelain", "linen"]
    : ["graphite", "ember", "aurora", "midnight", "cobalt"];
}

export type ApplyThemeOptions = {
  persist?: boolean;
  /** When true (default), align bg/fg presets with the resolved light/dark theme. */
  syncSurfaces?: boolean;
};

export function applyTheme(theme: Theme, style?: ThemeStyle, options: ApplyThemeOptions = {}): void {
  if (typeof document === "undefined") return;
  const { syncSurfaces = true } = options;
  const root = document.documentElement;
  const resolved = resolveTheme(theme);
  const nextStyle =
    style !== undefined ? normalizeThemeStyleForTheme(style, theme) : normalizeThemeStyleForTheme(currentStyle, theme);

  if (syncSurfaces) {
    syncSurfacePresetsToTheme(resolved);
  }

  root.setAttribute("data-theme", resolved);
  root.setAttribute("data-theme-style", nextStyle);
  currentTheme = theme;
  currentStyle = nextStyle;

  if (typeof window !== "undefined" && window.runtime) {
    if (theme === "auto") WindowSetSystemDefaultTheme();
    else if (theme === "light") WindowSetLightTheme();
    else WindowSetDarkTheme();
    if (resolved === "light") WindowSetBackgroundColour(251, 252, 254, 255);
    else WindowSetBackgroundColour(16, 16, 16, 255);
  }

  applyAppearancePrefs();
}

export function readLegacyThemePreference(): { theme: Theme; style: ThemeStyle; hasValue: boolean } {
  if (typeof localStorage === "undefined") {
    return { theme: DEFAULT_THEME, style: DEFAULT_LIGHT_STYLE, hasValue: false };
  }
  let rawTheme: string | null = null;
  let rawStyle: string | null = null;
  try {
    rawTheme = localStorage.getItem(THEME_KEY);
    rawStyle = localStorage.getItem(STYLE_KEY);
  } catch {
    return { theme: DEFAULT_THEME, style: DEFAULT_LIGHT_STYLE, hasValue: false };
  }
  const hasValue = rawTheme !== null || rawStyle !== null;
  let theme = DEFAULT_THEME;
  if (rawTheme) {
    try {
      theme = normalizeThemePreference(JSON.parse(rawTheme) as unknown);
    } catch {
      theme = normalizeThemePreference(rawTheme);
    }
  }
  const style = normalizeThemeStyleForTheme(rawStyle ?? undefined, theme);
  return { theme, style, hasValue };
}

export function clearLegacyThemePreference(): void {
  try {
    localStorage.removeItem(THEME_KEY);
    localStorage.removeItem(STYLE_KEY);
  } catch {
    /* ignore */
  }
}

export function initTheme(): void {
  applyTheme(getTheme(), getThemeStyle(), { syncSurfaces: false });
  installSystemThemeListener();
}
