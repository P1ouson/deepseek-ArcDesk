import { useEffect, useState } from "react";
import { app } from "../../lib/bridge";
import {
  CODE_FONT_SIZES,
  DIFF_MARKER_STYLES,
  applyAppearancePrefs,
  appearanceViewFromCurrentState,
  isBackgroundPreset,
  isForegroundPreset,
  loadBackgroundPreset,
  loadCodeFontSize,
  loadDiffMarkerStyle,
  loadForegroundPreset,
  saveBackgroundPreset,
  saveCodeFontSize,
  saveDiffMarkerStyle,
  saveForegroundPreset,
  saveTextSize,
  type BackgroundPreset,
  type CodeFontSize,
  type DiffMarkerStyle,
  type ForegroundPreset,
} from "../../lib/appearancePrefs";
import {
  applyTheme,
  getTheme,
  getThemeStyle,
  normalizeThemePreference,
  normalizeThemeStyleForTheme,
  type Theme,
  type ThemeStyle,
} from "../../lib/theme";
import { getTextSize, isTextSize, type TextSize } from "../../lib/textSize";
import type { SettingsView } from "../../lib/types";

export function useAppearanceSettingsState(
  s: SettingsView | null,
  apply: (fn: () => Promise<void>) => Promise<void>,
) {
  const [theme, setThemeState] = useState<Theme>(getTheme());
  const [themeStyle, setThemeStyleState] = useState<ThemeStyle>(() => getThemeStyle(getTheme()));
  const [textSize, setTextSizeState] = useState<TextSize>(getTextSize());
  const [backgroundPreset, setBackgroundPresetState] = useState<BackgroundPreset>(() => loadBackgroundPreset());
  const [foregroundPreset, setForegroundPresetState] = useState<ForegroundPreset>(() => loadForegroundPreset());
  const [codeFontSize, setCodeFontSizeState] = useState<CodeFontSize>(() => loadCodeFontSize());
  const [diffMarker, setDiffMarkerState] = useState<DiffMarkerStyle>(() => loadDiffMarkerStyle());

  useEffect(() => {
    if (!s) return;
    const nextTheme = normalizeThemePreference(s.desktopTheme);
    const nextStyle = normalizeThemeStyleForTheme(s.desktopThemeStyle, nextTheme);
    setThemeState(nextTheme);
    setThemeStyleState(nextStyle);
    applyTheme(nextTheme, nextStyle, { syncSurfaces: false });
  }, [s?.desktopTheme, s?.desktopThemeStyle]);

  useEffect(() => {
    if (!s?.desktopAppearance) return;
    const a = s.desktopAppearance;
    if (isBackgroundPreset(a.backgroundPreset)) setBackgroundPresetState(a.backgroundPreset);
    if (isForegroundPreset(a.foregroundPreset)) setForegroundPresetState(a.foregroundPreset);
    if (isTextSize(a.textSize)) setTextSizeState(a.textSize);
    if (CODE_FONT_SIZES.includes(a.codeFontSize as CodeFontSize)) setCodeFontSizeState(a.codeFontSize as CodeFontSize);
    if (DIFF_MARKER_STYLES.includes(a.diffMarker as DiffMarkerStyle)) setDiffMarkerState(a.diffMarker as DiffMarkerStyle);
  }, [s?.desktopAppearance]);

  const persistAppearanceConfig = () => app.SetDesktopAppearancePrefs(appearanceViewFromCurrentState());

  return {
    theme,
    themeStyle,
    backgroundPreset,
    foregroundPreset,
    textSize,
    codeFontSize,
    diffMarker,
    onTheme: (nextTheme: Theme) => {
      const nextStyle = normalizeThemeStyleForTheme(themeStyle, nextTheme);
      applyTheme(nextTheme, nextStyle, { syncSurfaces: true });
      setBackgroundPresetState(loadBackgroundPreset());
      setForegroundPresetState(loadForegroundPreset());
      setThemeState(nextTheme);
      setThemeStyleState(nextStyle);
      void apply(async () => {
        await app.SetDesktopAppearance(nextTheme, nextStyle);
        await persistAppearanceConfig();
      });
    },
    onThemeStyle: (nextStyle: ThemeStyle) => {
      applyTheme(theme, nextStyle, { syncSurfaces: false });
      setThemeStyleState(nextStyle);
      void apply(() => app.SetDesktopAppearance(theme, nextStyle));
    },
    onBackgroundPreset: (preset: BackgroundPreset) => {
      const nextForeground = saveBackgroundPreset(preset);
      setBackgroundPresetState(preset);
      setForegroundPresetState(nextForeground);
      applyAppearancePrefs();
      void apply(persistAppearanceConfig);
    },
    onForegroundPreset: (preset: ForegroundPreset) => {
      const nextForeground = saveForegroundPreset(preset);
      setForegroundPresetState(nextForeground);
      applyAppearancePrefs();
      void apply(persistAppearanceConfig);
    },
    onTextSize: (size: TextSize) => {
      saveTextSize(size);
      applyAppearancePrefs();
      setTextSizeState(size);
      void apply(persistAppearanceConfig);
    },
    onCodeFontSize: (size: CodeFontSize) => {
      saveCodeFontSize(size);
      setCodeFontSizeState(size);
      applyAppearancePrefs();
      void apply(persistAppearanceConfig);
    },
    onDiffMarker: (style: DiffMarkerStyle) => {
      saveDiffMarkerStyle(style);
      setDiffMarkerState(style);
      applyAppearancePrefs();
      void apply(persistAppearanceConfig);
    },
  };
}
