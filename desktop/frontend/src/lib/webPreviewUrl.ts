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
  return DEFAULT_PREVIEW_URL;
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
