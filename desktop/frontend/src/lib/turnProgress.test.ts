import { describe, expect, it } from "vitest";
import type { DictKey } from "./i18n";
import { deriveTurnProgress } from "./turnProgress";
import type { Item } from "./useController";

const t = (key: DictKey, vars?: Record<string, string | number>) => {
  if (key === "turnProgress.readingDetail" && vars) {
    return `files ${vars.done}/${vars.total}`;
  }
  const labels: Record<string, string> = {
    "turnProgress.segmented": "segmented",
    "turnProgress.reading": "reading",
    "turnProgress.scanning": "scanning",
    "turnProgress.thinking": "thinking",
    "turnProgress.filesGeneric": "files",
    "turnProgress.slowHint": "slow",
  };
  return labels[key as string] ?? key;
};

describe("deriveTurnProgress", () => {
  it("returns null when not running", () => {
    expect(deriveTurnProgress({ running: false, turnStartAt: 1000, items: [], t })).toBeNull();
  });

  it("prefers segmented read after truncation notice", () => {
    const items: Item[] = [
      { kind: "notice", id: "n1", level: "info", text: "tool output truncated: 1 of 2 bytes elided" },
      { kind: "tool", id: "t1", name: "read_file", args: "{}", readOnly: true, status: "running" },
    ];
    const p = deriveTurnProgress({ running: true, turnStartAt: Date.now() - 4000, items, t });
    expect(p?.phase).toBe("segmented");
  });

  it("reports reading with file counts after 3s", () => {
    const items: Item[] = [
      { kind: "tool", id: "t1", name: "read_file", args: '{"path":"a.go"}', readOnly: true, status: "done" },
      { kind: "tool", id: "t2", name: "read_file", args: '{"path":"b.go"}', readOnly: true, status: "running" },
    ];
    const p = deriveTurnProgress({
      running: true,
      turnStartAt: Date.now() - 5000,
      items,
      t,
    });
    expect(p?.phase).toBe("reading");
    expect(p?.detail).toBe("files 1/2");
  });
});
