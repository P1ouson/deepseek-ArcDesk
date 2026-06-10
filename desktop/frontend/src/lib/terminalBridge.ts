import type { TerminalStartResult } from "./types";

const TERMINAL_OUTPUT_EVENT = "terminal:output";
const TERMINAL_EXIT_EVENT = "terminal:exit";

function realApp() {
  return typeof window !== "undefined" ? window.go?.main?.App : undefined;
}

function bytesToBase64(data: string): string {
  const bytes = new TextEncoder().encode(data);
  let binary = "";
  for (let i = 0; i < bytes.length; i++) binary += String.fromCharCode(bytes[i]!);
  return btoa(binary);
}

let decodeWarned = false;

/** Decode base64 terminal payload; returns null and drops malformed chunks. */
export function decodeTerminalPayload(data: string): Uint8Array | null {
  try {
    const binary = atob(data);
    const out = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) out[i] = binary.charCodeAt(i);
    return out;
  } catch (err) {
    if (!decodeWarned) {
      console.warn("terminalBridge: dropping malformed terminal payload", err);
      decodeWarned = true;
    }
    return null;
  }
}

/** @internal test seam */
export function resetTerminalDecodeWarnForTests(): void {
  decodeWarned = false;
}

let mockTerminalSeq = 0;

export async function startTerminal(): Promise<TerminalStartResult> {
  const bindings = realApp();
  if (bindings?.StartTerminal) {
    return bindings.StartTerminal();
  }
  mockTerminalSeq += 1;
  return { id: `mock-term-${mockTerminalSeq}`, shell: "mock-shell", err: "" };
}

export function writeTerminal(sessionId: string, data: string): void {
  const bindings = realApp();
  if (bindings?.WriteTerminal) {
    void bindings.WriteTerminal(sessionId, bytesToBase64(data));
  }
}

export function resizeTerminal(sessionId: string, cols: number, rows: number): void {
  const bindings = realApp();
  if (bindings?.ResizeTerminal) {
    void bindings.ResizeTerminal(sessionId, cols, rows);
  }
}

export function closeTerminal(sessionId: string): void {
  if (!sessionId) return;
  const bindings = realApp();
  if (bindings?.CloseTerminal) {
    void bindings.CloseTerminal(sessionId);
  }
}

export function closeAllTerminals(): void {
  const bindings = realApp();
  if (bindings?.CloseTerminal) {
    void bindings.CloseTerminal("");
  }
}

export function onTerminalOutput(sessionId: string, cb: (data: Uint8Array) => void): () => void {
  if (realApp() && typeof window !== "undefined" && window.runtime) {
    return window.runtime.EventsOn(TERMINAL_OUTPUT_EVENT, (payload) => {
      const row = payload as { id?: string; data?: string };
      if (!row?.data || row.id !== sessionId) return;
      const decoded = decodeTerminalPayload(row.data);
      if (!decoded) return;
      cb(decoded);
    });
  }
  return () => {};
}

export function onTerminalExit(sessionId: string, cb: (code: number) => void): () => void {
  if (realApp() && typeof window !== "undefined" && window.runtime) {
    return window.runtime.EventsOn(TERMINAL_EXIT_EVENT, (payload) => {
      const row = payload as { id?: string; code?: number };
      if (row?.id !== sessionId) return;
      cb(typeof row?.code === "number" ? row.code : 0);
    });
  }
  return () => {};
}

export function onAnyTerminalExit(cb: (sessionId: string, code: number) => void): () => void {
  if (realApp() && typeof window !== "undefined" && window.runtime) {
    return window.runtime.EventsOn(TERMINAL_EXIT_EVENT, (payload) => {
      const row = payload as { id?: string; code?: number };
      if (!row?.id) return;
      cb(row.id, typeof row?.code === "number" ? row.code : 0);
    });
  }
  return () => {};
}
