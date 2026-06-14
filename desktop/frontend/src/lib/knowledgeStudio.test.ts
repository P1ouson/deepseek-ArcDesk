import { describe, expect, it } from "vitest";
import {
  cardTitle,
  filterBySection,
  isNegativeEntry,
  scoreEntry,
  searchAndSort,
  sectionCounts,
} from "./knowledgeStudio";
import type { KnowledgeEntry } from "./types";

const sample: KnowledgeEntry[] = [
  {
    id: "boot-test-mock-envaware",
    signature: "boot-test-mock-envaware",
    error: "boot test timeout on Windows",
    fix: "Mock envaware.Probe in boot_test.go",
    paths: ["internal/boot/boot_test.go"],
    confidence: "verified",
    hits: 4,
    kind: "fix",
    summary: "boot-test-mock-envaware: Mock envaware.Probe in boot_test.go",
  },
  {
    id: "avoid-runtime-direct",
    signature: "avoid-runtime-direct",
    error: "",
    fix: "Don't edit runtime/* directly — causes bootstrap fail",
    paths: ["internal/runtime"],
    confidence: "user_confirmed",
    hits: 2,
    kind: "convention",
    summary: "avoid-runtime-direct: Don't edit runtime/* directly",
  },
  {
    id: "old-entry",
    signature: "old-entry",
    error: "failed",
    fix: "old fix",
    confidence: "stale",
    hits: 1,
    kind: "fix",
    summary: "old-entry: old fix",
  },
];

describe("knowledgeStudio", () => {
  it("filters by section", () => {
    expect(filterBySection(sample, "fix")).toHaveLength(2);
    expect(filterBySection(sample, "stale")).toHaveLength(1);
    expect(filterBySection(sample, "convention")).toHaveLength(1);
  });

  it("detects negative entries", () => {
    expect(isNegativeEntry(sample[1])).toBe(true);
    expect(isNegativeEntry(sample[0])).toBe(false);
  });

  it("scores search matches higher", () => {
    const ranked = searchAndSort(sample, "boot windows", "all");
    expect(ranked[0]?.id).toBe("boot-test-mock-envaware");
    expect(scoreEntry(sample[0], "boot")).toBeGreaterThan(scoreEntry(sample[2], "boot"));
  });

  it("uses summary as card title when available", () => {
    expect(cardTitle(sample[0])).toBe("boot-test-mock-envaware: Mock envaware.Probe in boot_test.go");
  });

  it("counts sections", () => {
    expect(sectionCounts(sample).negative).toBe(1);
    expect(sectionCounts(sample).all).toBe(3);
  });
});
