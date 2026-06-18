import type { Translator } from "./i18n";

const PROVIDER_TRANSPORT_PREFIX =
  /^[\w.-]+:\s*(?:request failed|read stream|build request|decode stream|marshal request):\s*/i;
const PROVIDER_STATUS_PREFIX = /^[\w.-]+:\s*status\s*(\d{3})(?::\s*(.*))?$/i;
const OPERATIONAL_PREFIX = /^(?:fetch models|validate|config|rebuild|network|save|cannot switch model):\s*/i;

const EN = {
  requestCanceled: "Request canceled.",
  requestTimeout: "Request timed out. Please retry.",
  network: "Could not reach the API. Check the base URL, network, and proxy settings.",
  streamInterrupted: "Connection to the model was interrupted. Please retry.",
  auth: "Authentication failed (HTTP 401): your API key is missing, wrong, or expired.",
  balance: "Insufficient balance (HTTP 402): your account is out of credit.",
  rateLimited: "Rate limit reached (HTTP 429): too many requests. Please retry shortly.",
  server: "Server error (HTTP 500): the provider hit an internal fault. Please retry later.",
  busy: "Server busy (HTTP 503): the provider is overloaded. Please retry shortly.",
  badRequest: "Malformed request (HTTP 400): the request body was rejected.",
  unprocessable: "Invalid parameters (HTTP 422): a request parameter was rejected.",
} as const;

type ErrorKey = keyof typeof EN;

function L(key: ErrorKey, t?: Translator): string {
  return t ? t(`errors.${key}`) : EN[key];
}

function statusMessage(code: number, t?: Translator): string {
  switch (code) {
    case 400:
      return L("badRequest", t);
    case 401:
      return L("auth", t);
    case 402:
      return L("balance", t);
    case 422:
      return L("unprocessable", t);
    case 429:
      return L("rateLimited", t);
    case 500:
      return L("server", t);
    case 503:
      return L("busy", t);
    default:
      return "";
  }
}

function stripOperationalPrefixes(raw: string): string {
  let out = raw.trim();
  for (let i = 0; i < 6; i++) {
    const next = out.replace(OPERATIONAL_PREFIX, "").trim();
    if (next === out) break;
    out = next;
  }
  return out;
}

function stripHttpMethodUrl(raw: string): string {
  for (const method of ["Post ", "Get ", "Put ", "Patch ", "Delete "]) {
    if (!raw.startsWith(method) || !raw.slice(method.length).startsWith('"')) continue;
    const rest = raw.slice(method.length + 1);
    const close = rest.indexOf('"');
    if (close < 0) continue;
    const after = rest.slice(close + 1).trim();
    if (after.startsWith(":")) return after.slice(1).trim();
  }
  return raw;
}

function isNetworkFailure(lower: string): boolean {
  return /(?:^|\s)eof\b|connection reset|connection refused|broken pipe|tls:|dial tcp|i\/o timeout|network is unreachable|no such host|wsarecv/i.test(
    lower,
  );
}

function classifyTail(tail: string, t?: Translator): string {
  const lower = tail.toLowerCase();
  if (lower.includes("context canceled")) return L("requestCanceled", t);
  if (lower.includes("context deadline exceeded") || /\btimeout\b/i.test(lower)) return L("requestTimeout", t);
  if (lower.includes("unexpected eof")) return L("streamInterrupted", t);
  if (isNetworkFailure(lower)) return L("network", t);
  return "";
}

function humanizeStatusLine(raw: string, t?: Translator): string {
  const match = raw.match(PROVIDER_STATUS_PREFIX);
  if (!match) return "";
  const code = Number(match[1]);
  const msg = statusMessage(code, t);
  if (!msg) return "";
  const body = (match[2] ?? "").trim();
  if (body && (code === 400 || code === 422)) return `${msg}\n${body}`;
  return msg;
}

/** Normalize provider/transport errors for any UI surface. Idempotent on already-clean text. */
export function humanizeUserError(raw: string, t?: Translator): string {
  let msg = stripOperationalPrefixes(raw);
  const statusMsg = humanizeStatusLine(msg, t);
  if (statusMsg) return statusMsg;

  const strippedProvider = msg.replace(PROVIDER_TRANSPORT_PREFIX, "");
  const hadProviderPrefix = strippedProvider !== msg;
  msg = stripHttpMethodUrl(
    strippedProvider.replace(/^request failed:\s*/i, "").replace(/^read stream:\s*/i, "").replace(/^decode stream:\s*/i, "").trim(),
  );

  const classified = classifyTail(msg, t);
  if (classified) return classified;
  if (hadProviderPrefix && msg) return msg;

  const bare = stripOperationalPrefixes(raw).match(/^[\w.-]+:\s*(.+)$/i);
  if (bare) {
    const body = bare[1].trim();
    const bodyClassified = classifyTail(body, t) || humanizeStatusLine(body, t);
    if (bodyClassified) return bodyClassified;
    if (!/^(?:request failed|read stream|build request|decode stream|marshal request|status)\b/i.test(body)) {
      return body;
    }
  }

  return raw.trim();
}

/** Normalize unknown thrown/rejected values to a user-facing message string. */
export function toErrorMessage(err: unknown, fallback = "", t?: Translator): string {
  const raw =
    err instanceof Error ? err.message || fallback :
    typeof err === "string" ? err || fallback :
    err == null ? fallback :
    String(err);
  if (!raw) return fallback;
  const human = humanizeUserError(raw, t);
  return human || fallback;
}
