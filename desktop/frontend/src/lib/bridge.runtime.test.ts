import { describe, expect, it, beforeEach, afterEach, vi } from "vitest";
import { bridgeBindingSource, bridgeEventStreamSource } from "./bridge";
import { hasGoBinding, isRuntimeReady, isWailsRuntime } from "./runtime";

type TestWindow = Window & {
  go?: { main?: { App?: Record<string, unknown> } };
  runtime?: { EventsOn: ReturnType<typeof vi.fn> };
};

function setWindowShape(go: boolean, runtime: boolean) {
  const w = globalThis as unknown as { window: TestWindow };
  w.window = {
    go: go ? { main: { App: {} } } : undefined,
    runtime: runtime
      ? { EventsOn: vi.fn(() => () => {}) }
      : undefined,
  } as TestWindow;
}

describe("runtime readiness gate", () => {
  const originalWindow = globalThis.window;

  beforeEach(() => {
    vi.stubGlobal("window", originalWindow);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("A: no go and no runtime → mock mode", () => {
    setWindowShape(false, false);
    expect(isRuntimeReady()).toBe(false);
    expect(isWailsRuntime()).toBe(false);
    expect(hasGoBinding()).toBe(false);
    expect(bridgeBindingSource()).toBe("mock");
    expect(bridgeEventStreamSource()).toBe("mock");
  });

  it("B: go without runtime → not ready, no split-brain (mock IPC + deferred events)", () => {
    setWindowShape(true, false);
    expect(isRuntimeReady()).toBe(false);
    expect(isWailsRuntime()).toBe(false);
    expect(hasGoBinding()).toBe(true);
    expect(bridgeBindingSource()).toBe("mock");
    expect(bridgeEventStreamSource()).toBe("deferred");
  });

  it("C: go and runtime → runtime ready", () => {
    setWindowShape(true, true);
    expect(isRuntimeReady()).toBe(true);
    expect(isWailsRuntime()).toBe(true);
    expect(bridgeBindingSource()).toBe("wails");
    expect(bridgeEventStreamSource()).toBe("wails");
  });

  it("D: binding and event stream gates stay consistent", () => {
    for (const shape of [
      [false, false],
      [true, false],
      [true, true],
    ] as const) {
      setWindowShape(shape[0], shape[1]);
      const binding = bridgeBindingSource();
      const events = bridgeEventStreamSource();
      if (isRuntimeReady()) {
        expect(binding).toBe("wails");
        expect(events).toBe("wails");
      } else if (hasGoBinding()) {
        expect(binding).toBe("mock");
        expect(events).toBe("deferred");
      } else {
        expect(binding).toBe("mock");
        expect(events).toBe("mock");
      }
    }
  });
});

describe("onEvent subscribe branch", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetModules();
  });

  it("does not attach mock listeners when go exists without runtime", async () => {
    const eventsOn = vi.fn(() => () => {});
    vi.stubGlobal("window", {
      go: { main: { App: {} } },
      runtime: undefined,
      setInterval: (fn: () => void) => {
        fn();
        return 1;
      },
      clearInterval: vi.fn(),
    });

    vi.resetModules();
    const { onEvent, bridgeEventStreamSource } = await import("./bridge");
    expect(bridgeEventStreamSource()).toBe("deferred");

    const received: unknown[] = [];
    onEvent((e) => received.push(e));

    expect(received).toHaveLength(0);
    expect(eventsOn).not.toHaveBeenCalled();
  });
});
