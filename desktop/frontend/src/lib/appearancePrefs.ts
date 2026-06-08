import { DEFAULT_TEXT_SIZE, TEXT_SIZES, getTextSize, type TextSize } from "./textSize";
import type { DesktopAppearanceView } from "./types";

export const BACKGROUND_PRESETS = {
  paper: "#fbfbfa",
  white: "#ffffff",
  fog: "#f3f4f6",
  linen: "#faf8f5",
  studio: "#eef1f6",
  parchment: "#f3efe8",
  charcoal: "#191919",
  graphite: "#1e1e1e",
  slate: "#252526",
  midnight: "#0d1117",
  nightfall: "#12151c",
} as const;

export const FOREGROUND_PRESETS = {
  ink: "#2f3437",
  charcoal: "#1f1f1f",
  slate: "#4a5568",
  snow: "#ebebeb",
  silver: "#c9c9c9",
  white: "#ffffff",
} as const;

export type BackgroundPreset = keyof typeof BACKGROUND_PRESETS;
export type ForegroundPreset = keyof typeof FOREGROUND_PRESETS;
export type ResolvedTheme = "light" | "dark";

export const BACKGROUND_PRESET_IDS = Object.keys(BACKGROUND_PRESETS) as BackgroundPreset[];
export const FOREGROUND_PRESET_IDS = Object.keys(FOREGROUND_PRESETS) as ForegroundPreset[];

/** Light surface backgrounds — pair with dark foreground presets only. */
export const LIGHT_BACKGROUND_PRESETS: BackgroundPreset[] = ["paper", "white", "fog", "linen", "studio", "parchment"];
/** Dark surface backgrounds — pair with light foreground presets only. */
export const DARK_BACKGROUND_PRESETS: BackgroundPreset[] = ["charcoal", "graphite", "slate", "midnight", "nightfall"];
/** Dark text colors for light backgrounds. */
export const DARK_FOREGROUND_PRESETS: ForegroundPreset[] = ["ink", "charcoal", "slate"];
/** Light text colors for dark backgrounds. */
export const LIGHT_FOREGROUND_PRESETS: ForegroundPreset[] = ["snow", "silver", "white"];

const MIN_SURFACE_CONTRAST = 4.5;

export const CODE_FONT_SIZES = ["small", "default", "large", "xlarge"] as const;
export type CodeFontSize = (typeof CODE_FONT_SIZES)[number];

export const DIFF_MARKER_STYLES = ["background", "signs"] as const;
export type DiffMarkerStyle = (typeof DIFF_MARKER_STYLES)[number];

export const DEFAULT_CODE_FONT_SIZE: CodeFontSize = "default";
export const DEFAULT_DIFF_MARKER_STYLE: DiffMarkerStyle = "background";
/** Default dark surface: blue-steel studio canvas + white text. */
export const DEFAULT_BACKGROUND: BackgroundPreset = "nightfall";
export const DEFAULT_FOREGROUND: ForegroundPreset = "white";
/** Default light surface: cool studio paper + charcoal text. */
export const DEFAULT_LIGHT_BACKGROUND: BackgroundPreset = "studio";
export const DEFAULT_LIGHT_FOREGROUND: ForegroundPreset = "charcoal";

const KEYS = {
  bg: "ARCDESK.appearance.bg",
  fg: "ARCDESK.appearance.fg",
  codeFontSize: "ARCDESK.appearance.codeFontSize",
  diffMarker: "ARCDESK.appearance.diffMarker",
  migratedBw: "ARCDESK.appearance.bwDefaultV1",
  migratedStudioCanvas: "ARCDESK.appearance.studioCanvasV1",
  migratedToConfig: "ARCDESK.appearance.migratedToConfig",
} as const;

function read(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

function write(key: string, value: string | null): void {
  try {
    if (value == null) localStorage.removeItem(key);
    else localStorage.setItem(key, value);
  } catch {
    /* ignore */
  }
}

function isHexColor(value: string): boolean {
  return /^#[0-9a-fA-F]{6}$/.test(value);
}

function presetFromHex<T extends string>(hex: string, presets: Record<T, string>, fallback: T): T {
  const lower = hex.toLowerCase();
  for (const id of Object.keys(presets) as T[]) {
    if (presets[id].toLowerCase() === lower) return id;
  }
  return fallback;
}

export function readResolvedThemeFromDom(): ResolvedTheme {
  if (typeof document === "undefined") return "light";
  return document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "light";
}

export function themeDefaultBackgroundPreset(theme: ResolvedTheme): BackgroundPreset {
  return theme === "dark" ? DEFAULT_BACKGROUND : DEFAULT_LIGHT_BACKGROUND;
}

export function themeDefaultForegroundPreset(theme: ResolvedTheme): ForegroundPreset {
  return theme === "dark" ? DEFAULT_FOREGROUND : DEFAULT_LIGHT_FOREGROUND;
}

/** Move users off the old Reasonix-like charcoal/linen defaults to the studio canvas pair once. */
function migrateLegacyCharcoalDefault(): void {
  if (read(KEYS.migratedStudioCanvas)) return;
  const bg = read(KEYS.bg);
  const theme = readResolvedThemeFromDom();
  if (theme === "dark") {
    if (!bg || bg === "charcoal" || bg === "graphite") {
      write(KEYS.bg, DEFAULT_BACKGROUND);
    }
  } else if (!bg || bg === "linen" || bg === "paper") {
    write(KEYS.bg, DEFAULT_LIGHT_BACKGROUND);
  }
  write(KEYS.migratedStudioCanvas, "1");
}

/** Move users off the old paper/ink defaults to the current light studio/charcoal pair once. */
function migrateLegacyBrightDefaults(): void {
  if (read(KEYS.migratedBw)) return;
  const bg = read(KEYS.bg);
  const fg = read(KEYS.fg);
  const legacyPaperInk = (!bg || bg === "paper") && (!fg || fg === "ink");
  if (legacyPaperInk) {
    write(KEYS.bg, DEFAULT_LIGHT_BACKGROUND);
    write(KEYS.fg, DEFAULT_LIGHT_FOREGROUND);
  }
  write(KEYS.migratedBw, "1");
}

/** When the user switches light/dark theme, align surface presets so the theme visibly applies. */
export function syncSurfacePresetsToTheme(theme: ResolvedTheme): {
  background: BackgroundPreset;
  foreground: ForegroundPreset;
} {
  const background = themeDefaultBackgroundPreset(theme);
  const foreground = themeDefaultForegroundPreset(theme);
  write(KEYS.bg, background);
  write(KEYS.fg, foreground);
  return { background, foreground };
}

export function isBackgroundPreset(value: unknown): value is BackgroundPreset {
  return typeof value === "string" && value in BACKGROUND_PRESETS;
}

export function isForegroundPreset(value: unknown): value is ForegroundPreset {
  return typeof value === "string" && value in FOREGROUND_PRESETS;
}

export function backgroundColorFor(preset: BackgroundPreset): string {
  return BACKGROUND_PRESETS[preset];
}

export function foregroundColorFor(preset: ForegroundPreset): string {
  return FOREGROUND_PRESETS[preset];
}

function contrastRatio(bg: string, fg: string): number {
  const l1 = luminance(bg);
  const l2 = luminance(fg);
  const lighter = Math.max(l1, l2);
  const darker = Math.min(l1, l2);
  return (lighter + 0.05) / (darker + 0.05);
}

export function isLightBackgroundPreset(preset: BackgroundPreset): boolean {
  return LIGHT_BACKGROUND_PRESETS.includes(preset);
}

export function isLightForegroundPreset(preset: ForegroundPreset): boolean {
  return LIGHT_FOREGROUND_PRESETS.includes(preset);
}

export function backgroundPresetsForTheme(theme: ResolvedTheme): BackgroundPreset[] {
  return theme === "light" ? LIGHT_BACKGROUND_PRESETS : DARK_BACKGROUND_PRESETS;
}

export function foregroundPresetsForBackground(background: BackgroundPreset): ForegroundPreset[] {
  return isLightBackgroundPreset(background) ? DARK_FOREGROUND_PRESETS : LIGHT_FOREGROUND_PRESETS;
}

function defaultForegroundForBackground(background: BackgroundPreset): ForegroundPreset {
  return isLightBackgroundPreset(background) ? DEFAULT_LIGHT_FOREGROUND : DEFAULT_FOREGROUND;
}

function bestForegroundForBackground(background: BackgroundPreset, candidates: ForegroundPreset[]): ForegroundPreset {
  const bg = backgroundColorFor(background);
  let best = candidates[0]!;
  let bestRatio = 0;
  for (const preset of candidates) {
    const ratio = contrastRatio(bg, foregroundColorFor(preset));
    if (ratio > bestRatio) {
      best = preset;
      bestRatio = ratio;
    }
  }
  return best;
}

export function normalizeSurfacePair(
  background: BackgroundPreset,
  foreground: ForegroundPreset,
): { background: BackgroundPreset; foreground: ForegroundPreset } {
  const allowedForegrounds = foregroundPresetsForBackground(background);
  if (!allowedForegrounds.includes(foreground)) {
    return { background, foreground: defaultForegroundForBackground(background) };
  }
  const bgHex = backgroundColorFor(background);
  const fgHex = foregroundColorFor(foreground);
  if (contrastRatio(bgHex, fgHex) >= MIN_SURFACE_CONTRAST) {
    return { background, foreground };
  }
  return {
    background,
    foreground: bestForegroundForBackground(background, allowedForegrounds),
  };
}

function persistSurfacePair(background: BackgroundPreset, foreground: ForegroundPreset): void {
  write(KEYS.bg, background);
  write(KEYS.fg, foreground);
}

export function loadBackgroundPreset(): BackgroundPreset {
  const v = read(KEYS.bg);
  if (isBackgroundPreset(v)) return v;
  if (v && isHexColor(v)) return presetFromHex(v, BACKGROUND_PRESETS, themeDefaultBackgroundPreset(readResolvedThemeFromDom()));
  return themeDefaultBackgroundPreset(readResolvedThemeFromDom());
}

export function loadForegroundPreset(): ForegroundPreset {
  const v = read(KEYS.fg);
  if (isForegroundPreset(v)) return v;
  if (v && isHexColor(v)) return presetFromHex(v, FOREGROUND_PRESETS, themeDefaultForegroundPreset(readResolvedThemeFromDom()));
  return themeDefaultForegroundPreset(readResolvedThemeFromDom());
}

export function loadCodeFontSize(): CodeFontSize {
  const v = read(KEYS.codeFontSize);
  return CODE_FONT_SIZES.includes(v as CodeFontSize) ? (v as CodeFontSize) : DEFAULT_CODE_FONT_SIZE;
}

export function loadDiffMarkerStyle(): DiffMarkerStyle {
  const v = read(KEYS.diffMarker);
  return DIFF_MARKER_STYLES.includes(v as DiffMarkerStyle) ? (v as DiffMarkerStyle) : DEFAULT_DIFF_MARKER_STYLE;
}

export function saveBackgroundPreset(preset: BackgroundPreset): ForegroundPreset {
  const pair = normalizeSurfacePair(preset, loadForegroundPreset());
  persistSurfacePair(pair.background, pair.foreground);
  return pair.foreground;
}

export function saveForegroundPreset(preset: ForegroundPreset): ForegroundPreset {
  const pair = normalizeSurfacePair(loadBackgroundPreset(), preset);
  persistSurfacePair(pair.background, pair.foreground);
  return pair.foreground;
}

export function saveCodeFontSize(size: CodeFontSize): void {
  write(KEYS.codeFontSize, size === DEFAULT_CODE_FONT_SIZE ? null : size);
}

export function saveDiffMarkerStyle(style: DiffMarkerStyle): void {
  write(KEYS.diffMarker, style);
}

export function saveTextSize(size: TextSize): void {
  try {
    if (size === DEFAULT_TEXT_SIZE) localStorage.removeItem("ARCDESK-text-size");
    else localStorage.setItem("ARCDESK-text-size", size);
  } catch {
    /* ignore */
  }
}

const SURFACE_VAR_KEYS = [
  "--bg",
  "--bg-soft",
  "--bg-elev",
  "--bg-elev-2",
  "--fg",
  "--fg-dim",
  "--fg-faint",
  "--sidebar-bg",
  "--sidebar-hover",
  "--chat-bg",
  "--code-canvas-bg",
  "--workspace-preview-bg",
  "--workspace-files-bg",
  "--workspace-files-hover",
  "--panel",
  "--panel-2",
  "--panel-inset",
  "--bg-muted",
  "--fg-muted",
  "--hover",
  "--border",
  "--border-soft",
] as const;

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace("#", "");
  return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
}

function rgbToHex(r: number, g: number, b: number): string {
  const clamp = (v: number) => Math.round(Math.max(0, Math.min(255, v)));
  return `#${[clamp(r), clamp(g), clamp(b)].map((v) => v.toString(16).padStart(2, "0")).join("")}`;
}

function mixHex(a: string, b: string, t: number): string {
  const [ar, ag, ab] = hexToRgb(a);
  const [br, bg, bb] = hexToRgb(b);
  return rgbToHex(ar + (br - ar) * t, ag + (bg - ag) * t, ab + (bb - ab) * t);
}

function luminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex).map((v) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

/** Code workspace canvas — cooler tint, visually distinct from neutral gray panels. */
function buildCodeCanvasBg(bg: string, dark: boolean): string {
  if (dark) {
    const steel = mixHex(mixHex(bg, "#1a2438", 0.34), "#141c2e", 0.18);
    return mixHex(steel, "#ffffff", 0.045);
  }
  return mixHex(mixHex(bg, "#c8d4e4", 0.2), "#ffffff", 0.32);
}

export function buildSurfaceVars(bg: string, fg: string): Record<string, string> {
  const dark = luminance(bg) < 0.4;
  const white = "#ffffff";
  const bgSoft = mixHex(bg, fg, dark ? 0.07 : 0.035);
  const bgElev = dark ? mixHex(bg, white, 0.06) : mixHex(bg, white, 0.42);
  const bgElev2 = mixHex(bgSoft, bgElev, 0.45);
  const fgDim = mixHex(fg, bg, 0.32);
  const fgFaint = mixHex(fg, bg, 0.52);
  const sidebarHover = mixHex(bgSoft, fg, dark ? 0.09 : 0.06);
  const panelInset = mixHex(bgSoft, fg, dark ? 0.14 : 0.1);
  const codeCanvasBg = buildCodeCanvasBg(bg, dark);
  const [fr, fgG, fb] = hexToRgb(fg);

  return {
    "--bg": bg,
    "--bg-soft": bgSoft,
    "--bg-elev": bgElev,
    "--bg-elev-2": bgElev2,
    "--fg": fg,
    "--fg-dim": fgDim,
    "--fg-faint": fgFaint,
    "--sidebar-bg": bgSoft,
    "--sidebar-hover": sidebarHover,
    "--chat-bg": codeCanvasBg,
    "--code-canvas-bg": codeCanvasBg,
    "--workspace-preview-bg": bgElev,
    "--workspace-files-bg": bgSoft,
    "--workspace-files-hover": sidebarHover,
    "--panel": bgElev,
    "--panel-2": bgElev2,
    "--panel-inset": panelInset,
    "--bg-muted": bgSoft,
    "--fg-muted": fgDim,
    "--hover": sidebarHover,
    "--border": `rgba(${fr}, ${fgG}, ${fb}, ${dark ? 0.1 : 0.11})`,
    "--border-soft": `rgba(${fr}, ${fgG}, ${fb}, ${dark ? 0.06 : 0.07})`,
  };
}

function applyTextSizeAttr(root: HTMLElement, size: TextSize): void {
  if (size === DEFAULT_TEXT_SIZE) root.removeAttribute("data-text-size");
  else root.setAttribute("data-text-size", size);
}

function applyCodeFontSizeAttr(root: HTMLElement, size: CodeFontSize): void {
  if (size === DEFAULT_CODE_FONT_SIZE) root.removeAttribute("data-code-font-size");
  else root.setAttribute("data-code-font-size", size);
}

function applyDiffMarkerAttr(root: HTMLElement, style: DiffMarkerStyle): void {
  root.setAttribute("data-diff-marker", style);
}

export function applyAppearancePrefs(): void {
  if (typeof document === "undefined") return;
  migrateLegacyCharcoalDefault();
  migrateLegacyBrightDefaults();
  const root = document.documentElement;

  const storedBg = loadBackgroundPreset();
  const storedFg = loadForegroundPreset();
  const pair = normalizeSurfacePair(storedBg, storedFg);
  if (pair.background !== storedBg || pair.foreground !== storedFg) {
    persistSurfacePair(pair.background, pair.foreground);
  }

  const bg = backgroundColorFor(pair.background);
  const fg = foregroundColorFor(pair.foreground);
  const surfaces = buildSurfaceVars(bg, fg);
  for (const key of SURFACE_VAR_KEYS) {
    root.style.setProperty(key, surfaces[key]!);
  }
  root.style.colorScheme = luminance(bg) < 0.4 ? "dark" : "light";
  root.setAttribute("data-appearance-surfaces", "1");

  const textSize = getTextSize();
  applyTextSizeAttr(root, textSize);

  const codeSize = loadCodeFontSize();
  applyCodeFontSizeAttr(root, codeSize);

  const diffStyle = loadDiffMarkerStyle();
  applyDiffMarkerAttr(root, diffStyle);
}

export function initAppearancePrefs(): void {
  applyAppearancePrefs();
}

function resolveBackgroundFromConfig(view: DesktopAppearanceView | undefined | null): BackgroundPreset {
  if (view && isBackgroundPreset(view.backgroundPreset)) return view.backgroundPreset;
  return loadBackgroundPreset();
}

function resolveForegroundFromConfig(view: DesktopAppearanceView | undefined | null): ForegroundPreset {
  if (view && isForegroundPreset(view.foregroundPreset)) return view.foregroundPreset;
  return loadForegroundPreset();
}

function resolveTextSizeFromConfig(view: DesktopAppearanceView | undefined | null): TextSize {
  const size = view?.textSize;
  return size && (TEXT_SIZES as readonly string[]).includes(size) ? (size as TextSize) : getTextSize();
}

function resolveCodeFontSizeFromConfig(view: DesktopAppearanceView | undefined | null): CodeFontSize {
  const size = view?.codeFontSize;
  return size && CODE_FONT_SIZES.includes(size as CodeFontSize) ? (size as CodeFontSize) : loadCodeFontSize();
}

function resolveDiffMarkerFromConfig(view: DesktopAppearanceView | undefined | null): DiffMarkerStyle {
  const style = view?.diffMarker;
  return style && DIFF_MARKER_STYLES.includes(style as DiffMarkerStyle)
    ? (style as DiffMarkerStyle)
    : loadDiffMarkerStyle();
}

/** Apply desktop appearance from config (source of truth) and mirror to local cache. */
export function syncAppearanceFromSettings(view: DesktopAppearanceView | undefined | null): void {
  if (typeof document === "undefined") return;
  const pair = normalizeSurfacePair(resolveBackgroundFromConfig(view), resolveForegroundFromConfig(view));
  persistSurfacePair(pair.background, pair.foreground);
  saveTextSize(resolveTextSizeFromConfig(view));
  saveCodeFontSize(resolveCodeFontSizeFromConfig(view));
  saveDiffMarkerStyle(resolveDiffMarkerFromConfig(view));
  applyAppearancePrefs();
}

export function appearanceViewFromCurrentState(): DesktopAppearanceView {
  return {
    backgroundPreset: loadBackgroundPreset(),
    foregroundPreset: loadForegroundPreset(),
    textSize: getTextSize(),
    codeFontSize: loadCodeFontSize(),
    diffMarker: loadDiffMarkerStyle(),
  };
}

export function readLocalAppearanceForMigration(): { hasValue: boolean; view: DesktopAppearanceView } {
  if (typeof localStorage === "undefined" || read(KEYS.migratedToConfig) === "1") {
    return { hasValue: false, view: appearanceViewFromCurrentState() };
  }
  const bg = read(KEYS.bg);
  const fg = read(KEYS.fg);
  const textSize = read("ARCDESK-text-size");
  const codeFontSize = read(KEYS.codeFontSize);
  const diffMarker = read(KEYS.diffMarker);
  const hasValue = !!(bg || fg || textSize || codeFontSize || diffMarker);
  return {
    hasValue,
    view: {
      backgroundPreset: isBackgroundPreset(bg) ? bg : "",
      foregroundPreset: isForegroundPreset(fg) ? fg : "",
      textSize: textSize && (TEXT_SIZES as readonly string[]).includes(textSize) ? textSize : "default",
      codeFontSize:
        codeFontSize && CODE_FONT_SIZES.includes(codeFontSize as CodeFontSize) ? codeFontSize : "default",
      diffMarker:
        diffMarker && DIFF_MARKER_STYLES.includes(diffMarker as DiffMarkerStyle) ? diffMarker : "background",
    },
  };
}

export function markLocalAppearanceMigrated(): void {
  write(KEYS.migratedToConfig, "1");
}
