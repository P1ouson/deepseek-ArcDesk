import { useEffect, useState } from "react";

/** Wails Go binding injected (may precede window.runtime). */
export function hasGoBinding(): boolean {
  return typeof window !== "undefined" && Boolean(window.go?.main?.App);
}

/** True when both Go IPC and Wails runtime event stream are available. */
export function isRuntimeReady(): boolean {
  return hasGoBinding() && typeof window !== "undefined" && Boolean(window.runtime);
}

/** Fully connected Wails desktop shell (go + runtime). */
export function isWailsRuntime(): boolean {
  return isRuntimeReady();
}

const RUNTIME_POLL_MS = 50;

/** Re-renders when window.runtime becomes available after window.go. */
export function useRuntimeReady(): boolean {
  const [ready, setReady] = useState(() => isRuntimeReady());
  useEffect(() => {
    if (ready) return;
    const id = window.setInterval(() => {
      if (isRuntimeReady()) setReady(true);
    }, RUNTIME_POLL_MS);
    return () => window.clearInterval(id);
  }, [ready]);
  return ready;
}
