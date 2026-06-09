import { describe, expect, it } from "vitest";
import { currencySymbol, formatMoney, formatTokens } from "./formatMoney";

describe("formatMoney", () => {
  it("formats CNY amounts", () => {
    expect(formatMoney(0.5, "CNY")).toBe("¥0.5000");
    expect(formatMoney(12.34, "CNY")).toBe("¥12.34");
  });

  it("formats USD amounts", () => {
    expect(formatMoney(1.2, "USD")).toBe("$1.20");
  });

  it("returns zero placeholder for missing amounts", () => {
    expect(formatMoney(undefined, "CNY")).toBe("¥0.0000");
    expect(formatMoney(-1, "CNY")).toBe("¥0.0000");
  });
});

describe("currencySymbol", () => {
  it("normalizes common currency codes", () => {
    expect(currencySymbol("cny")).toBe("¥");
    expect(currencySymbol("usd")).toBe("$");
    expect(currencySymbol("¥")).toBe("¥");
  });
});

describe("formatTokens", () => {
  it("abbreviates thousands", () => {
    expect(formatTokens(1500)).toBe("1.5k");
    expect(formatTokens(1000)).toBe("1k");
  });

  it("keeps small counts as-is", () => {
    expect(formatTokens(42)).toBe("42");
  });
});
