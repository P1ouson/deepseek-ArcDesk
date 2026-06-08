import type { DesktopCodeReviewView } from "./types";

export type CodeReviewDefaultScope = "all" | "session" | "git";

const SCOPE_KEY = "ARCDESK.codeReview.defaultScope";
const SECURITY_KEY = "ARCDESK.codeReview.securityByDefault";
const MIGRATED_KEY = "ARCDESK.codeReview.migratedToConfig";

let currentScope: CodeReviewDefaultScope = "all";
let currentSecurity = false;

function normalizeScope(scope: string | undefined): CodeReviewDefaultScope {
  return scope === "session" || scope === "git" ? scope : "all";
}

export function syncCodeReviewSettings(view: DesktopCodeReviewView | undefined | null): void {
  currentScope = normalizeScope(view?.defaultScope);
  currentSecurity = view?.securityByDefault === true;
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("ARCDESK:code-review-settings"));
  }
}

export function getCodeReviewDefaultScope(): CodeReviewDefaultScope {
  return currentScope;
}

export function getCodeReviewSecurityByDefault(): boolean {
  return currentSecurity;
}

/** @deprecated use getCodeReviewDefaultScope after config sync */
export function loadCodeReviewDefaultScope(): CodeReviewDefaultScope {
  if (typeof localStorage === "undefined") return currentScope;
  const raw = localStorage.getItem(SCOPE_KEY);
  return raw === "session" || raw === "git" ? raw : currentScope;
}

/** @deprecated use getCodeReviewSecurityByDefault after config sync */
export function loadCodeReviewSecurityByDefault(): boolean {
  if (typeof localStorage === "undefined") return currentSecurity;
  return localStorage.getItem(SECURITY_KEY) === "1" || currentSecurity;
}

export function readLocalCodeReviewForMigration(): {
  hasValue: boolean;
  scope: CodeReviewDefaultScope;
  security: boolean;
} {
  if (typeof localStorage === "undefined" || localStorage.getItem(MIGRATED_KEY) === "1") {
    return { hasValue: false, scope: "all", security: false };
  }
  const scopeRaw = localStorage.getItem(SCOPE_KEY);
  const securityRaw = localStorage.getItem(SECURITY_KEY);
  const hasScope = scopeRaw === "session" || scopeRaw === "git";
  const hasSecurity = securityRaw === "1";
  if (!hasScope && !hasSecurity) {
    return { hasValue: false, scope: "all", security: false };
  }
  return {
    hasValue: true,
    scope: hasScope ? scopeRaw : "all",
    security: hasSecurity,
  };
}

export function markLocalCodeReviewMigrated(): void {
  try {
    localStorage.setItem(MIGRATED_KEY, "1");
    localStorage.removeItem(SCOPE_KEY);
    localStorage.removeItem(SECURITY_KEY);
  } catch {
    /* ignore */
  }
}

export function saveCodeReviewDefaultScope(scope: CodeReviewDefaultScope): void {
  currentScope = scope;
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("ARCDESK:code-review-settings"));
  }
}

export function saveCodeReviewSecurityByDefault(on: boolean): void {
  currentSecurity = on;
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("ARCDESK:code-review-settings"));
  }
}
