export type WebPreviewDecision = "allow" | "blocked" | "confirm";

export interface WebPreviewValidation {
  decision: WebPreviewDecision;
  url: string;
  reason?: "invalid" | "unsafe-scheme" | "external";
}

const DEFAULT_PREVIEW_URL = "http://localhost:5173";

const BLOCKED_SCHEMES = new Set(["javascript", "data", "file", "vbscript", "about", "blob"]);

function isLocalHost(hostname: string): boolean {
  const host = hostname.toLowerCase();
  return host === "localhost" || host === "127.0.0.1" || host === "[::1]" || host === "::1";
}

export function defaultPreviewUrl(): string {
  if (typeof window !== "undefined") {
    const { protocol, hostname, port } = window.location;
    const host = hostname.toLowerCase();
    const onLocalDev =
      (host === "localhost" || host === "127.0.0.1") &&
      (port === "5173" || port === "5174" || port === "4173");
    if (onLocalDev && (protocol === "http:" || protocol === "https:")) {
      return window.location.origin;
    }
  }
  return DEFAULT_PREVIEW_URL;
}

/** Best-effort probe — false when nothing is listening (e.g. pnpm dev not running). */
export async function probePreviewReachable(raw: string, timeoutMs = 2800): Promise<boolean> {
  const trimmed = raw.trim();
  if (!trimmed) return false;
  let href: string;
  try {
    href = new URL(trimmed.includes("://") ? trimmed : `http://${trimmed}`).href;
  } catch {
    return false;
  }
  const ctrl = new AbortController();
  const timer = window.setTimeout(() => ctrl.abort(), timeoutMs);
  try {
    await fetch(href, { method: "GET", signal: ctrl.signal, cache: "no-store" });
    return true;
  } catch {
    return false;
  } finally {
    window.clearTimeout(timer);
  }
}

export function validatePreviewUrl(raw: string): WebPreviewValidation {
  const trimmed = raw.trim();
  if (!trimmed) {
    return { decision: "blocked", url: "", reason: "invalid" };
  }

  let parsed: URL;
  try {
    parsed = new URL(trimmed.includes("://") ? trimmed : `http://${trimmed}`);
  } catch {
    return { decision: "blocked", url: trimmed, reason: "invalid" };
  }

  const scheme = parsed.protocol.replace(":", "").toLowerCase();
  if (BLOCKED_SCHEMES.has(scheme)) {
    return { decision: "blocked", url: parsed.href, reason: "unsafe-scheme" };
  }
  if (scheme !== "http" && scheme !== "https") {
    return { decision: "blocked", url: parsed.href, reason: "unsafe-scheme" };
  }

  if (isLocalHost(parsed.hostname)) {
    return { decision: "allow", url: parsed.href };
  }

  return { decision: "confirm", url: parsed.href, reason: "external" };
}

/** iframe sandbox: scripts/forms/modals for dev apps; no top navigation or popups. */
export const PREVIEW_IFRAME_SANDBOX =
  "allow-scripts allow-same-origin allow-forms allow-modals";
