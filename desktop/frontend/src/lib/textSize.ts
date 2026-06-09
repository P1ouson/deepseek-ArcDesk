export const TEXT_SIZES = ["small", "default", "large", "xlarge"] as const;

export type TextSize = (typeof TEXT_SIZES)[number];

export const DEFAULT_TEXT_SIZE: TextSize = "default";

const TEXT_SIZE_KEY = "ARCDESK-text-size";

export function isTextSize(value: unknown): value is TextSize {
  return typeof value === "string" && (TEXT_SIZES as readonly string[]).includes(value);
}

export function getTextSize(): TextSize {
  const stored = typeof localStorage !== "undefined" ? localStorage.getItem(TEXT_SIZE_KEY) : null;
  return isTextSize(stored) ? stored : DEFAULT_TEXT_SIZE;
}
