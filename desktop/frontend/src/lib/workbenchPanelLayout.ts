/** Keep in sync with `.workbench__resizer` flex-basis in styles.css (4px). */
export const PANEL_RESIZER_WIDTH = 4;

export const STUDIO_TOOL_RAIL_WIDTH = 52;

/** Minimum chat column width when preview and/or dock are open. */
export const WORKBENCH_ROW_CHAT_MIN = 420;

export type WorkbenchRowChrome = {
  previewOpen: boolean;
  dockOpen: boolean;
  toolRail: boolean;
};

export function workbenchRowWidth(mainWidth: number, chrome: Pick<WorkbenchRowChrome, "toolRail">): number {
  const rail = chrome.toolRail ? STUDIO_TOOL_RAIL_WIDTH : 0;
  return Math.max(0, Math.round(mainWidth) - rail);
}

export function panelResizerChrome(chrome: Pick<WorkbenchRowChrome, "previewOpen" | "dockOpen">): number {
  let total = 0;
  if (chrome.previewOpen) total += PANEL_RESIZER_WIDTH;
  if (chrome.dockOpen) total += PANEL_RESIZER_WIDTH;
  return total;
}

/** Max preview width that still leaves room for chat (and an open dock). */
export function maxFilePreviewWidth(
  rowWidth: number,
  dockWidth: number,
  chrome: WorkbenchRowChrome,
): number {
  const dock = chrome.dockOpen ? Math.max(0, Math.round(dockWidth)) : 0;
  return Math.max(
    0,
    rowWidth - WORKBENCH_ROW_CHAT_MIN - panelResizerChrome(chrome) - dock,
  );
}

/** Max dock width that still leaves room for chat (and an open preview). */
export function maxDockPanelWidth(
  rowWidth: number,
  previewWidth: number,
  chrome: WorkbenchRowChrome,
): number {
  const preview = chrome.previewOpen ? Math.max(0, Math.round(previewWidth)) + PANEL_RESIZER_WIDTH : 0;
  return Math.max(0, rowWidth - WORKBENCH_ROW_CHAT_MIN - preview - (chrome.dockOpen ? PANEL_RESIZER_WIDTH : 0));
}

/**
 * Persist preview width after drag. Row budget is enforced live during pointermove
 * (partner panel frozen); commit only applies global min/max clamps.
 */
export function commitPreviewResize(
  previewDragged: number,
  dockWidth: number,
  clampPreview: (width: number) => number,
  clampDock: (width: number) => number,
): { preview: number; dock: number } {
  return { preview: clampPreview(previewDragged), dock: clampDock(dockWidth) };
}

/**
 * Persist dock width after drag. Row budget is enforced live during pointermove
 * (partner panel frozen); commit only applies global min/max clamps.
 */
export function commitDockResize(
  previewWidth: number,
  dockDragged: number,
  clampPreview: (width: number) => number,
  clampDock: (width: number) => number,
): { preview: number; dock: number } {
  return { preview: clampPreview(previewWidth), dock: clampDock(dockDragged) };
}
