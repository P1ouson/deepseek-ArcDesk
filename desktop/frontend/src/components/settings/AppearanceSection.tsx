import {
  CODE_FONT_SIZES,
  DIFF_MARKER_STYLES,
  backgroundColorFor,
  backgroundPresetsForTheme,
  foregroundColorFor,
  foregroundPresetsForBackground,
  type BackgroundPreset,
  type CodeFontSize,
  type DiffMarkerStyle,
  type ForegroundPreset,
} from "../../lib/appearancePrefs";
import {
  getResolvedTheme,
  stylesForTheme,
  type Theme,
  type ThemeStyle,
} from "../../lib/theme";
import { TEXT_SIZES, type TextSize } from "../../lib/textSize";
import { useT } from "../../lib/i18n";
import { StudioSelect } from "../StudioSelect";
import { SettingsBlock } from "../settingsPrimitives";

function colorSwatch(hex: string) {
  return <span className="settings-color-swatch" style={{ background: hex }} aria-hidden />;
}

function backgroundPresetName(id: BackgroundPreset, t: ReturnType<typeof useT>): string {
  return t(`settings.bg.${id}`);
}

function foregroundPresetName(id: ForegroundPreset, t: ReturnType<typeof useT>): string {
  return t(`settings.fg.${id}`);
}

function diffMarkerName(style: DiffMarkerStyle, t: ReturnType<typeof useT>): string {
  switch (style) {
    case "background":
      return t("settings.diffMarkerBackground");
    case "signs":
      return t("settings.diffMarkerSigns");
  }
}

function themeStyleName(style: ThemeStyle, t: ReturnType<typeof useT>): string {
  switch (style) {
    case "graphite":
      return t("settings.style.graphite");
    case "ember":
      return t("settings.style.ember");
    case "aurora":
      return t("settings.style.aurora");
    case "midnight":
      return t("settings.style.midnight");
    case "cobalt":
      return t("settings.style.cobalt");
    case "sandstone":
      return t("settings.style.sandstone");
    case "porcelain":
      return t("settings.style.porcelain");
    case "linen":
      return t("settings.style.linen");
    case "glacier":
      return t("settings.style.glacier");
  }
}

function themeName(theme: Theme, t: ReturnType<typeof useT>): string {
  switch (theme) {
    case "auto":
      return t("settings.themeAuto");
    case "light":
      return t("settings.themeLight");
    case "dark":
      return t("settings.themeDark");
  }
}

function textSizeName(size: TextSize, t: ReturnType<typeof useT>): string {
  switch (size) {
    case "small":
      return t("settings.textSizeSmall");
    case "default":
      return t("settings.textSizeDefault");
    case "large":
      return t("settings.textSizeLarge");
    case "xlarge":
      return t("settings.textSizeXLarge");
  }
}

export function AppearanceSection({
  theme,
  themeStyle,
  backgroundPreset,
  foregroundPreset,
  textSize,
  codeFontSize,
  diffMarker,
  onTheme,
  onThemeStyle,
  onBackgroundPreset,
  onForegroundPreset,
  onTextSize,
  onCodeFontSize,
  onDiffMarker,
}: {
  theme: Theme;
  themeStyle: ThemeStyle;
  backgroundPreset: BackgroundPreset;
  foregroundPreset: ForegroundPreset;
  textSize: TextSize;
  codeFontSize: CodeFontSize;
  diffMarker: DiffMarkerStyle;
  onTheme: (t: Theme) => void;
  onThemeStyle: (style: ThemeStyle) => void;
  onBackgroundPreset: (preset: BackgroundPreset) => void;
  onForegroundPreset: (preset: ForegroundPreset) => void;
  onTextSize: (size: TextSize) => void;
  onCodeFontSize: (size: CodeFontSize) => void;
  onDiffMarker: (style: DiffMarkerStyle) => void;
}) {
  const t = useT();
  const themeOptions: Theme[] = ["auto", "light", "dark"];
  const accentOptions = stylesForTheme(theme);
  const resolvedTheme = getResolvedTheme(theme);
  const backgroundIds = backgroundPresetsForTheme(resolvedTheme);
  const safeBackground = backgroundIds.includes(backgroundPreset) ? backgroundPreset : backgroundIds[0]!;
  const foregroundIds = foregroundPresetsForBackground(safeBackground);
  const safeForeground = foregroundIds.includes(foregroundPreset) ? foregroundPreset : foregroundIds[0]!;
  const backgroundOptions = backgroundIds.map((id) => ({
    value: id,
    label: backgroundPresetName(id, t),
    icon: colorSwatch(backgroundColorFor(id)),
  }));
  const foregroundOptions = foregroundIds.map((id) => ({
    value: id,
    label: foregroundPresetName(id, t),
    icon: colorSwatch(foregroundColorFor(id)),
  }));

  return (
    <>
      <SettingsBlock title={t("settings.theme")}>
        <div className="set-seg set-seg--compact">
          {themeOptions.map((opt) => (
            <button
              key={opt}
              type="button"
              className={`set-seg__btn${theme === opt ? " set-seg__btn--on" : ""}`}
              onClick={() => onTheme(opt)}
            >
              {themeName(opt, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
      <SettingsBlock title={t("settings.themeStyle")}>
        <div className="settings-accent-grid">
          {accentOptions.map((style) => (
            <button
              key={style}
              type="button"
              className={`settings-accent-swatch${themeStyle === style ? " settings-accent-swatch--on" : ""}`}
              data-style={style}
              onClick={() => onThemeStyle(style)}
            >
              <span className="settings-accent-swatch__dot" />
              {themeStyleName(style, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
      <SettingsBlock title={t("settings.backgroundColor")}>
        <StudioSelect
          value={safeBackground}
          onChange={(value) => onBackgroundPreset(value as BackgroundPreset)}
          options={backgroundOptions}
        />
      </SettingsBlock>
      <SettingsBlock title={t("settings.foregroundColor")}>
        <StudioSelect
          value={safeForeground}
          onChange={(value) => onForegroundPreset(value as ForegroundPreset)}
          options={foregroundOptions}
        />
      </SettingsBlock>
      <SettingsBlock title={t("settings.uiFontSize")}>
        <div className="set-seg set-seg--compact">
          {TEXT_SIZES.map((size) => (
            <button
              key={size}
              type="button"
              className={`set-seg__btn${textSize === size ? " set-seg__btn--on" : ""}`}
              onClick={() => onTextSize(size)}
            >
              {textSizeName(size, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
      <SettingsBlock title={t("settings.codeFontSize")}>
        <div className="set-seg set-seg--compact">
          {CODE_FONT_SIZES.map((size) => (
            <button
              key={size}
              type="button"
              className={`set-seg__btn${codeFontSize === size ? " set-seg__btn--on" : ""}`}
              onClick={() => onCodeFontSize(size)}
            >
              {textSizeName(size, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
      <SettingsBlock title={t("settings.diffMarker")}>
        <p className="settings-block__card-lead">{t("settings.diffMarkerHint")}</p>
        <div className="set-seg set-seg--compact">
          {DIFF_MARKER_STYLES.map((style) => (
            <button
              key={style}
              type="button"
              className={`set-seg__btn${diffMarker === style ? " set-seg__btn--on" : ""}`}
              onClick={() => onDiffMarker(style)}
            >
              {diffMarkerName(style, t)}
            </button>
          ))}
        </div>
      </SettingsBlock>
    </>
  );
}
