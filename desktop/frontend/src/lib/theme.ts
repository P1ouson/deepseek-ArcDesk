// theme.ts — System / Light / Dark appearance for Reasonix Desktop.
// Applies data-theme on <html> and syncs native Wails window chrome.

import {
  WindowSetDarkTheme,
  WindowSetLightTheme,
  WindowSetSystemDefaultTheme,
  WindowSetBackgroundColour,
} from "../../wailsjs/runtime/runtime";

export type Theme = "auto" | "light" | "dark";
export type ResolvedTheme = Exclude<Theme, "auto">;

const DEFAULT_THEME: Theme = "light";
const THEME_KEY = "reasonix-theme";
let currentTheme: Theme = DEFAULT_THEME;
let systemThemeListenerInstalled = false;

function resolveTheme(theme: Theme): ResolvedTheme {
  if (theme === "light" || theme === "dark") return theme;
  if (typeof window !== "undefined" && window.matchMedia?.("(prefers-color-scheme: light)").matches) return "light";
  return "dark";
}

function syncSystemTheme(theme: Theme): void {
  if (typeof document === "undefined") return;
  if (theme !== "auto") return;
  document.documentElement.setAttribute("data-theme", resolveTheme(theme));
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

/** @deprecated Theme styles removed — kept for Settings migration compatibility. */
export type ThemeStyle = "default";
export const THEME_STYLES = ["default"] as const;

export function isThemeStyle(_value: unknown): _value is ThemeStyle {
  return false;
}

export function getTheme(): Theme {
  return currentTheme;
}

export function getResolvedTheme(theme: Theme = getTheme()): ResolvedTheme {
  return resolveTheme(theme);
}

export function defaultStyleForTheme(_theme: Theme): ThemeStyle {
  return "default";
}

export function themeForStyle(_style: ThemeStyle): ResolvedTheme {
  return getResolvedTheme();
}

export function getThemeStyle(_theme: Theme = getTheme()): ThemeStyle {
  return "default";
}

export function normalizeThemeStyleForTheme(_style: string | undefined, _theme: Theme): ThemeStyle {
  return "default";
}

export function applyTheme(theme: Theme, _style?: ThemeStyle, _options: { persist?: boolean } = {}): void {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  root.removeAttribute("data-theme-style");
  const resolved = resolveTheme(theme);
  root.setAttribute("data-theme", resolved);
  currentTheme = theme;

  if (typeof window !== "undefined" && window.runtime) {
    if (theme === "auto") WindowSetSystemDefaultTheme();
    else if (theme === "light") WindowSetLightTheme();
    else WindowSetDarkTheme();
  }

  if (theme === "auto") syncSystemTheme(theme);
}

export function readLegacyThemePreference(): { theme: Theme; style: ThemeStyle; hasValue: boolean } {
  if (typeof localStorage === "undefined") return { theme: DEFAULT_THEME, style: "default", hasValue: false };
  let rawTheme: string | null = null;
  let rawStyle: string | null = null;
  try {
    rawTheme = localStorage.getItem(THEME_KEY);
    rawStyle = localStorage.getItem("reasonix-theme-style");
  } catch {
    return { theme: DEFAULT_THEME, style: "default", hasValue: false };
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
  return { theme, style: "default", hasValue };
}

export function clearLegacyThemePreference(): void {
  try {
    localStorage.removeItem(THEME_KEY);
    localStorage.removeItem("reasonix-theme-style");
  } catch {
    /* ignore */
  }
}

export function initTheme(): void {
  applyTheme(getTheme(), "default", { persist: false });
  installSystemThemeListener();

  if (typeof window !== "undefined" && window.runtime) {
    const resolved = getResolvedTheme(getTheme());
    if (resolved === "light") {
      WindowSetBackgroundColour(251, 252, 254, 255);
    } else {
      WindowSetBackgroundColour(16, 16, 16, 255);
    }
  }
}
