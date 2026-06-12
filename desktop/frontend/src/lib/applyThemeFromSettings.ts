import type { DesktopAppearanceView, SettingsView } from "./types";
import { applyTheme, normalizeThemePreference, normalizeThemeStyleForTheme } from "./theme";
import { syncAppearanceFromSettings } from "./appearancePrefs";
import { syncDesktopGitSettings } from "./desktopGitPrefs";
import { syncCodeReviewSettings } from "./codeReviewPrefs";

export type ThemeSyncSource = "boot" | "settings" | "slash";

const SYNC_SURFACES: Record<ThemeSyncSource, boolean> = {
  boot: false,
  settings: true,
  slash: true,
};

/** Single entry for theme + desktop prefs after settings load or mutation. */
export function applyThemeFromSettings(settings: SettingsView, source: ThemeSyncSource = "settings"): void {
  const nextTheme = normalizeThemePreference(settings.desktopTheme);
  const nextStyle = normalizeThemeStyleForTheme(settings.desktopThemeStyle, nextTheme);
  applyTheme(nextTheme, nextStyle, { syncSurfaces: SYNC_SURFACES[source], persist: source !== "boot" });
  syncAppearanceFromSettings(settings.desktopAppearance);
  syncDesktopGitSettings(settings.desktopGit);
  syncCodeReviewSettings(settings.desktopCodeReview);
}

export type { DesktopAppearanceView };
