import { useCallback, useEffect, useState } from "react";
import { AlertTriangle, Monitor, RefreshCw, ShieldCheck } from "lucide-react";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { PreviewURLValidation } from "../lib/types";
import {
  defaultPreviewUrl,
  PREVIEW_IFRAME_SANDBOX,
  validatePreviewUrl,
} from "../lib/webPreviewUrl";

export function BrowserPanel() {
  const t = useT();
  const [url, setUrl] = useState(() => defaultPreviewUrl());
  const [src, setSrc] = useState(() => defaultPreviewUrl());
  const [status, setStatus] = useState<PreviewURLValidation>(() => ({
    decision: "allow",
    url: defaultPreviewUrl(),
    strict: false,
  }));
  const [blockedMessage, setBlockedMessage] = useState("");

  const resolveUrl = useCallback(async (raw: string): Promise<PreviewURLValidation> => {
    try {
      return await app.ValidatePreviewURL(raw);
    } catch {
      const fallback = validatePreviewUrl(raw);
      return {
        decision: fallback.decision,
        url: fallback.url,
        reason: fallback.reason,
        strict: false,
      };
    }
  }, []);

  useEffect(() => {
    void (async () => {
      const next = await resolveUrl(defaultPreviewUrl());
      setStatus(next);
    })();
  }, [resolveUrl]);

  const applyUrl = useCallback(
    async (raw: string, skipConfirm = false) => {
      const next = await resolveUrl(raw);
      setStatus(next);
      if (next.decision === "blocked") {
        setBlockedMessage(
          next.reason === "unsafe-scheme"
            ? t("browser.blockedScheme")
            : next.reason === "not-in-profile"
              ? t("browser.blockedProfile")
              : t("browser.blockedInvalid"),
        );
        return;
      }
      if (next.decision === "confirm" && !skipConfirm) {
        const ok = window.confirm(t("browser.externalConfirm", { url: next.url }));
        if (!ok) return;
      }
      setBlockedMessage("");
      setUrl(next.url);
      setSrc(next.url);
    },
    [resolveUrl, t],
  );

  const reload = useCallback(() => {
    setSrc((current) => {
      const base = current.split("#")[0] ?? current;
      return `${base}${base.includes("?") ? "&" : "?"}_preview=${Date.now()}`;
    });
  }, []);

  return (
    <div className="right-dock__browser">
      <div className="right-dock__browser-bar">
        <input
          value={url}
          onChange={(event) => setUrl(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") void applyUrl(url);
          }}
          placeholder={t("rightDock.urlPlaceholder")}
          spellCheck={false}
        />
        <div className="right-dock__browser-actions-inline">
          <button type="button" onClick={() => void applyUrl(url)}>
            {t("rightDock.browserGo")}
          </button>
          <button type="button" className="right-dock__browser-icon-btn" onClick={reload} aria-label={t("browser.reload")}>
            <RefreshCw size={14} />
          </button>
        </div>
      </div>

      <div className="right-dock__browser-meta">
        {status.strict ? (
          <span className="right-dock__browser-badge right-dock__browser-badge--warn">
            <ShieldCheck size={12} />
            {t("browser.sandboxStrict")}
          </span>
        ) : status.decision === "allow" && isLocalUrl(status.url) ? (
          <span className="right-dock__browser-badge right-dock__browser-badge--safe">
            <ShieldCheck size={12} />
            {t("browser.sandboxLocal")}
          </span>
        ) : status.decision === "confirm" ? (
          <span className="right-dock__browser-badge right-dock__browser-badge--warn">
            <AlertTriangle size={12} />
            {t("browser.sandboxExternal")}
          </span>
        ) : (
          <span className="right-dock__browser-badge">
            <Monitor size={12} />
            {t("browser.sandboxHint")}
          </span>
        )}
      </div>

      {blockedMessage ? (
        <div className="right-dock__browser-blocked">{blockedMessage}</div>
      ) : (
        <iframe
          key={src}
          title={t("browser.title")}
          src={src}
          className="right-dock__iframe"
          sandbox={PREVIEW_IFRAME_SANDBOX}
          referrerPolicy="no-referrer"
          loading="lazy"
        />
      )}
    </div>
  );
}

function isLocalUrl(raw: string): boolean {
  try {
    const host = new URL(raw).hostname.toLowerCase();
    return host === "localhost" || host === "127.0.0.1" || host === "[::1]" || host === "::1";
  } catch {
    return false;
  }
}
