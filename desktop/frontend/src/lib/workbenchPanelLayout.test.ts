import { describe, expect, it } from "vitest";
import {
  commitDockResize,
  commitPreviewResize,
  fitStudioRightPanels,
  maxStudioDockWidth,
  maxStudioPreviewWidth,
  PANEL_RESIZER_WIDTH,
  STUDIO_TOOL_RAIL_WIDTH,
  studioRightPanelsBudget,
  studioSinglePanelTargetWidth,
} from "./workbenchPanelLayout";

const bothOpen = { previewOpen: true, dockOpen: true, toolRail: true };
const previewOnly = { previewOpen: true, dockOpen: false, toolRail: true };

describe("workbenchPanelLayout", () => {
  it("allocates 40% of layout width to right panels minus tool rail", () => {
    const layout = 1600;
    expect(studioRightPanelsBudget(layout, true)).toBe(Math.round(layout * 0.4) - STUDIO_TOOL_RAIL_WIDTH);
    expect(studioSinglePanelTargetWidth(layout, true)).toBe(
      studioRightPanelsBudget(layout, true) - PANEL_RESIZER_WIDTH,
    );
  });

  it("uses the full right budget for a single open panel by default", () => {
    const layout = 1600;
    const budget = studioRightPanelsBudget(layout, true);
    const fitted = fitStudioRightPanels(280, 496, layout, previewOnly, {
      previewMin: 280,
      previewMax: 960,
      dockMin: 280,
      dockMax: 800,
    });
    expect(fitted.preview).toBe(budget - PANEL_RESIZER_WIDTH);
    expect(fitted.dock).toBe(0);
  });

  it("honors user-sized single panels within the right budget", () => {
    const layout = 1600;
    const fitted = fitStudioRightPanels(420, 496, layout, previewOnly, {
      previewMin: 280,
      previewMax: 960,
      dockMin: 280,
      dockMax: 800,
    }, { previewUserSized: true });
    expect(fitted.preview).toBe(420);
  });

  it("caps preview and dock so both fit inside the right budget", () => {
    const layout = 1600;
    const budget = studioRightPanelsBudget(layout, true);
    const fitted = fitStudioRightPanels(620, 496, layout, bothOpen, {
      previewMin: 280,
      previewMax: 960,
      dockMin: 280,
      dockMax: 800,
    });
    expect(fitted.preview + fitted.dock + PANEL_RESIZER_WIDTH * 2).toBeLessThanOrEqual(budget);
    expect(fitted.preview).toBeGreaterThan(0);
    expect(fitted.dock).toBeGreaterThan(0);
  });

  it("gives a single open panel the full right budget", () => {
    const layout = 1600;
    const target = studioSinglePanelTargetWidth(layout, true);
    const fitted = fitStudioRightPanels(target, 496, layout, previewOnly, {
      previewMin: 280,
      previewMax: 960,
      dockMin: 280,
      dockMax: 800,
    });
    expect(fitted.preview).toBe(target);
    expect(fitted.dock).toBe(0);
  });

  it("respects partner panel when computing max preview width", () => {
    const layout = 1600;
    const dock = 320;
    const maxPreview = maxStudioPreviewWidth(layout, dock, bothOpen);
    expect(maxPreview + dock + PANEL_RESIZER_WIDTH * 2).toBeLessThanOrEqual(studioRightPanelsBudget(layout, true));
    expect(maxStudioDockWidth(layout, maxPreview, bothOpen)).toBeGreaterThanOrEqual(0);
  });

  it("keeps dragged preview width on commit without rebalancing dock", () => {
    const clamp = (n: number) => Math.round(n);
    const committed = commitPreviewResize(520, 360, clamp, clamp);
    expect(committed.preview).toBe(520);
    expect(committed.dock).toBe(360);
  });

  it("splits file-tree preview and dock at 1.5:2.5 inside the right budget", () => {
    const layout = 1600;
    const budget = studioRightPanelsBudget(layout, true);
    const available = budget - PANEL_RESIZER_WIDTH * 2;
    const fitted = fitStudioRightPanels(280, 496, layout, bothOpen, {
      previewMin: 280,
      previewMax: 960,
      dockMin: 280,
      dockMax: 800,
    }, { splitRatio: { dock: 1.5, preview: 2.5 } });
    expect(fitted.preview + fitted.dock).toBe(available);
    expect(fitted.dock).toBe(Math.round((available * 1.5) / 4));
    expect(fitted.preview).toBe(available - fitted.dock);
  });

  it("keeps dragged dock width on commit without rebalancing preview", () => {
    const clamp = (n: number) => Math.round(n);
    const committed = commitDockResize(400, 420, clamp, clamp);
    expect(committed.dock).toBe(420);
    expect(committed.preview).toBe(400);
  });
});
