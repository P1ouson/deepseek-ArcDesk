import { hasGoBinding } from "./runtime";

export type RuntimeIngestItem = {
  kind: "console" | "go_log" | "network" | "state";
  level?: "info" | "warn" | "error";
  source?: string;
  message: string;
  meta?: Record<string, string>;
};

type ConsoleLevel = "log" | "info" | "warn" | "error" | "debug";

function levelForConsole(method: ConsoleLevel): RuntimeIngestItem["level"] {
  switch (method) {
    case "warn":
      return "warn";
    case "error":
      return "error";
    default:
      return "info";
  }
}

function formatConsoleArgs(args: unknown[]): string {
  return args
    .map((a) => {
      if (typeof a === "string") return a;
      try {
        return JSON.stringify(a);
      } catch {
        return String(a);
      }
    })
    .join(" ");
}

function postBatch(tabId: string, items: RuntimeIngestItem[]) {
  if (!items.length || !tabId) return;
  const app = (window as unknown as { go?: { main?: { App?: { IngestRuntime?: (tab: string, json: string) => Promise<void> } } } }).go?.main?.App;
  if (!app?.IngestRuntime) return;
  void app.IngestRuntime(tabId, JSON.stringify(items)).catch(() => {});
}

export function installRuntimeObserve(tabId: string) {
  if (typeof window === "undefined" || !hasGoBinding()) return () => {};

  const queue: RuntimeIngestItem[] = [];
  let flushTimer: ReturnType<typeof setTimeout> | null = null;

  const flush = () => {
    if (!queue.length) return;
    const batch = queue.splice(0, queue.length);
    postBatch(tabId, batch);
  };

  const enqueue = (item: RuntimeIngestItem) => {
    queue.push(item);
    if (flushTimer != null) return;
    flushTimer = setTimeout(() => {
      flushTimer = null;
      flush();
    }, 250);
  };

  const methods: ConsoleLevel[] = ["log", "info", "warn", "error", "debug"];
  const orig: Partial<Record<ConsoleLevel, (...args: unknown[]) => void>> = {};
  for (const m of methods) {
    orig[m] = console[m].bind(console);
    console[m] = (...args: unknown[]) => {
      enqueue({
        kind: "console",
        level: levelForConsole(m),
        source: "console." + m,
        message: formatConsoleArgs(args),
      });
      orig[m]?.(...args);
    };
  }

  const origFetch = window.fetch.bind(window);
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const started = performance.now();
    try {
      const res = await origFetch(input, init);
      enqueue({
        kind: "network",
        level: res.ok ? "info" : "error",
        source: "fetch",
        message: `${init?.method ?? "GET"} ${url} ${res.status}`,
        meta: {
          url,
          status: String(res.status),
          duration_ms: String(Math.round(performance.now() - started)),
        },
      });
      return res;
    } catch (err) {
      enqueue({
        kind: "network",
        level: "error",
        source: "fetch",
        message: `${init?.method ?? "GET"} ${url} failed: ${String(err)}`,
        meta: { url },
      });
      throw err;
    }
  };

  const stateTimer = setInterval(() => {
    enqueue({
      kind: "state",
      level: "info",
      source: "webview",
      message: "snapshot",
      meta: {
        path: location.pathname + location.search,
        visibility: document.visibilityState,
        online: String(navigator.onLine),
        user_agent: navigator.userAgent.slice(0, 120),
      },
    });
  }, 5000);

  enqueue({
    kind: "state",
    level: "info",
    source: "webview",
    message: "runtime_observe_installed",
    meta: { tab_id: tabId },
  });

  return () => {
    if (flushTimer != null) clearTimeout(flushTimer);
    clearInterval(stateTimer);
    flush();
    for (const m of methods) {
      if (orig[m]) console[m] = orig[m]!;
    }
    window.fetch = origFetch;
  };
}
