import { describe, expect, it } from "vitest";
import { usageChartBarHeightPx, USAGE_CHART_BAR_MAX_PX } from "./usageActivity";

describe("usageChartBarHeightPx", () => {
  it("returns pixel heights scaled to the chart area", () => {
    expect(usageChartBarHeightPx(0, 100)).toBe(4);
    expect(usageChartBarHeightPx(50, 100)).toBe(Math.round(0.5 * USAGE_CHART_BAR_MAX_PX));
    expect(usageChartBarHeightPx(100, 100)).toBe(USAGE_CHART_BAR_MAX_PX);
  });

  it("enforces a minimum visible bar height", () => {
    expect(usageChartBarHeightPx(1, 10_000)).toBe(8);
  });
});
