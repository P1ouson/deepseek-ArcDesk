export const DEFAULT_PREVIEW_LOGICAL_WIDTH = 1280;
export const DEFAULT_PREVIEW_LOGICAL_HEIGHT = 800;

export type BrowserPreviewZoomMode = "fit" | "manual";

export function computePreviewFitScale(
  containerWidth: number,
  containerHeight: number,
  contentWidth: number,
  contentHeight: number,
): number {
  if (containerWidth <= 0 || containerHeight <= 0) return 1;
  if (contentWidth <= 0 || contentHeight <= 0) return 1;
  return Math.min(containerWidth / contentWidth, containerHeight / contentHeight);
}

export function clampPreviewScale(scale: number): number {
  return Math.min(3, Math.max(0.15, scale));
}

/** Same-origin iframe only; cross-origin returns null. */
export function measureIframeDocumentSize(iframe: HTMLIFrameElement): { width: number; height: number } | null {
  try {
    const doc = iframe.contentDocument;
    if (!doc) return null;
    const root = doc.documentElement;
    const body = doc.body;
    const width = Math.max(
      root?.scrollWidth ?? 0,
      root?.clientWidth ?? 0,
      body?.scrollWidth ?? 0,
      body?.clientWidth ?? 0,
    );
    const height = Math.max(
      root?.scrollHeight ?? 0,
      root?.clientHeight ?? 0,
      body?.scrollHeight ?? 0,
      body?.clientHeight ?? 0,
    );
    if (width <= 0 || height <= 0) return null;
    return { width, height };
  } catch {
    return null;
  }
}
