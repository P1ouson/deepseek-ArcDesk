import { describe, expect, it } from "vitest";
import {
  commitDockResize,
  commitPreviewResize,
  maxDockPanelWidth,
  maxFilePreviewWidth,
  PANEL_RESIZER_WIDTH,
} from "./workbenchPanelLayout";

const bothOpen = { previewOpen: true, dockOpen: true, toolRail: true };

describe("workbenchPanelLayout", () => {
  it("reserves chat minimum and both resizers when preview and dock are open", () => {
    const row = 1400;
    const preview = 620;
    const dock = 360;
    expect(maxFilePreviewWidth(row, dock, bothOpen)).toBe(row - 420 - dock - PANEL_RESIZER_WIDTH * 2);
    expect(maxDockPanelWidth(row, preview, bothOpen)).toBe(
      row - 420 - preview - PANEL_RESIZER_WIDTH * 2,
    );
  });

  it("keeps preview and dock budgets consistent", () => {
    const row = 1280;
    const preview = 500;
    const dock = 320;
    const maxPreview = maxFilePreviewWidth(row, dock, bothOpen);
    const maxDock = maxDockPanelWidth(row, preview, bothOpen);
    expect(preview).toBeLessThanOrEqual(maxPreview);
    expect(dock).toBeLessThanOrEqual(maxDock);
    expect(preview + dock + 420 + PANEL_RESIZER_WIDTH * 2).toBeLessThanOrEqual(row);
  });

  it("keeps dragged preview width on commit without rebalancing dock", () => {
    const clamp = (n: number) => Math.round(n);
    const committed = commitPreviewResize(520, 360, clamp, clamp);
    expect(committed.preview).toBe(520);
    expect(committed.dock).toBe(360);
  });

  it("keeps dragged dock width on commit without rebalancing preview", () => {
    const clamp = (n: number) => Math.round(n);
    const committed = commitDockResize(400, 420, clamp, clamp);
    expect(committed.dock).toBe(420);
    expect(committed.preview).toBe(400);
  });
});
