/** Keep in sync with `.workbench__resizer` flex-basis in styles.css (4px). */
export const PANEL_RESIZER_WIDTH = 4;

export const STUDIO_TOOL_RAIL_WIDTH = 52;

/** Minimum chat column width when preview and/or dock are open. */
export const WORKBENCH_ROW_CHAT_MIN = 420;

/** Studio three-column target when left drawer + right panel are open: 2 : 4 : 4. */
export const STUDIO_LAYOUT_RATIOS = { left: 0.2, center: 0.4, right: 0.4 } as const;

const STUDIO_DRAWER_MIN_WIDTH = 200;

export function studioDrawerWidth(layoutWidth: number, railWidth: number, drawerOpen: boolean): number {
  if (!drawerOpen || layoutWidth <= 0) return 0;
  const leftBudget = Math.round(layoutWidth * STUDIO_LAYOUT_RATIOS.left);
  return Math.max(STUDIO_DRAWER_MIN_WIDTH, leftBudget - railWidth);
}

export function studioRightPanelWidth(layoutWidth: number): number {
  if (layoutWidth <= 0) return 0;
  return Math.round(layoutWidth * STUDIO_LAYOUT_RATIOS.right);
}

/** Right-side panel area (preview + dock + resizers), excluding the tool rail. */
export function studioRightPanelsBudget(layoutWidth: number, toolRail = true): number {
  const rail = toolRail ? STUDIO_TOOL_RAIL_WIDTH : 0;
  return Math.max(0, studioRightPanelWidth(layoutWidth) - rail);
}

export function studioPanelResizerCount(previewOpen: boolean, dockOpen: boolean): number {
  if (previewOpen && dockOpen) return 2;
  if (previewOpen || dockOpen) return 1;
  return 0;
}

export function studioPanelResizerWidth(previewOpen: boolean, dockOpen: boolean): number {
  return studioPanelResizerCount(previewOpen, dockOpen) * PANEL_RESIZER_WIDTH;
}

/** Default width for a single open right panel (preview or dock). */
/** When previewing from the file tree, split the right budget between dock and preview. */
export const STUDIO_FILE_TREE_SPLIT = { dock: 1.5, preview: 2.5 } as const;

export function studioFileTreeSplitWidths(
  layoutWidth: number,
  toolRail = true,
): { dock: number; preview: number } {
  const resizerWidth = studioPanelResizerWidth(true, true);
  const available = Math.max(0, studioRightPanelsBudget(layoutWidth, toolRail) - resizerWidth);
  const total = STUDIO_FILE_TREE_SPLIT.dock + STUDIO_FILE_TREE_SPLIT.preview;
  const dock = Math.round((available * STUDIO_FILE_TREE_SPLIT.dock) / total);
  return { dock, preview: available - dock };
}

/** Default width for a single open right panel (preview or dock). */
export function studioSinglePanelTargetWidth(layoutWidth: number, toolRail = true): number {
  return Math.max(0, studioRightPanelsBudget(layoutWidth, toolRail) - PANEL_RESIZER_WIDTH);
}

export function maxStudioPreviewWidth(
  layoutWidth: number,
  dockWidth: number,
  chrome: WorkbenchRowChrome,
): number {
  const dock = chrome.dockOpen ? Math.max(0, Math.round(dockWidth)) : 0;
  return Math.max(
    0,
    studioRightPanelsBudget(layoutWidth, chrome.toolRail) -
      studioPanelResizerWidth(chrome.previewOpen, chrome.dockOpen) -
      dock,
  );
}

export function maxStudioDockWidth(
  layoutWidth: number,
  previewWidth: number,
  chrome: WorkbenchRowChrome,
): number {
  const preview = chrome.previewOpen ? Math.max(0, Math.round(previewWidth)) : 0;
  return Math.max(
    0,
    studioRightPanelsBudget(layoutWidth, chrome.toolRail) -
      studioPanelResizerWidth(chrome.previewOpen, chrome.dockOpen) -
      preview,
  );
}

export type StudioPanelBounds = {
  previewMin: number;
  previewMax: number;
  dockMin: number;
  dockMax: number;
};

function clampPanel(width: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, Math.round(width)));
}

export type StudioPanelFitOptions = {
  /** User dragged preview narrower than the 40% budget. */
  previewUserSized?: boolean;
  /** User dragged dock narrower than the 40% budget. */
  dockUserSized?: boolean;
  /** Fixed dock:preview split when both panels are open (e.g. file-tree preview 1.5:2.5). */
  splitRatio?: { dock: number; preview: number };
};

/**
 * Fit saved preview/dock widths into the studio right budget (40% of window).
 * Single open panel defaults to the full budget unless the user has resized it.
 */
export function fitStudioRightPanels(
  savedPreview: number,
  savedDock: number,
  layoutWidth: number,
  chrome: WorkbenchRowChrome,
  bounds: StudioPanelBounds,
  options: StudioPanelFitOptions = {},
): { preview: number; dock: number } {
  const resizerWidth = studioPanelResizerWidth(chrome.previewOpen, chrome.dockOpen);
  const available = Math.max(0, studioRightPanelsBudget(layoutWidth, chrome.toolRail) - resizerWidth);

  if (!chrome.previewOpen && !chrome.dockOpen) {
    return { preview: 0, dock: 0 };
  }

  if (chrome.previewOpen && !chrome.dockOpen) {
    const preview = options.previewUserSized
      ? clampPanel(Math.min(savedPreview, available), bounds.previewMin, bounds.previewMax)
      : available;
    return { preview, dock: 0 };
  }

  if (!chrome.previewOpen && chrome.dockOpen) {
    const dock = options.dockUserSized
      ? clampPanel(Math.min(savedDock, available), bounds.dockMin, bounds.dockMax)
      : available;
    return { preview: 0, dock };
  }

  if (options.splitRatio && chrome.previewOpen && chrome.dockOpen) {
    const total = options.splitRatio.dock + options.splitRatio.preview;
    const dock = Math.round((available * options.splitRatio.dock) / total);
    const preview = available - dock;
    return { preview, dock };
  }

  let preview = clampPanel(savedPreview, bounds.previewMin, bounds.previewMax);
  let dock = clampPanel(savedDock, bounds.dockMin, bounds.dockMax);
  const sum = Math.max(0, savedPreview) + Math.max(0, savedDock);
  if (sum > 0) {
    preview = Math.round((Math.max(0, savedPreview) / sum) * available);
    dock = available - preview;
  } else {
    preview = Math.floor(available / 2);
    dock = available - preview;
  }

  if (dock < bounds.dockMin) {
    dock = bounds.dockMin;
    preview = available - dock;
  }
  if (preview < bounds.previewMin) {
    preview = bounds.previewMin;
    dock = available - preview;
  }

  preview = clampPanel(preview, bounds.previewMin, Math.min(bounds.previewMax, available));
  dock = clampPanel(available - preview, bounds.dockMin, bounds.dockMax);
  preview = Math.max(0, available - dock);

  return { preview, dock };
}

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

function clampPreviewWidth(width: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, Math.round(width)));
}

/**
 * Collapsed preview uses the saved width within the row budget; expanded takes the
 * maximum allowed width so expand never shrinks the column.
 */
export function resolvePreviewColumnRenderWidth(
  expanded: boolean,
  savedWidth: number,
  rowWidth: number,
  dockWidth: number,
  chrome: WorkbenchRowChrome,
  bounds: { min: number; max: number } = { min: 280, max: 960 },
): number {
  const fitPreview = (desired: number) =>
    clampPreviewWidth(
      Math.min(desired, maxFilePreviewWidth(rowWidth, dockWidth, chrome)),
      bounds.min,
      bounds.max,
    );
  const collapsed = fitPreview(savedWidth);
  if (!expanded) return collapsed;
  const maxAllowed = maxFilePreviewWidth(rowWidth, dockWidth, chrome);
  return clampPreviewWidth(Math.max(collapsed, maxAllowed), bounds.min, bounds.max);
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
