import { useCallback, useEffect, useRef, useState } from "react";
import {
  AlertTriangle,
  ArrowRight,
  Maximize2,
  Minimize2,
  Monitor,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import { app } from "../lib/bridge";
import { confirmAction } from "../lib/confirmAction";
import { useT } from "../lib/i18n";
import type { PreviewURLValidation } from "../lib/types";
import {
  defaultPreviewUrl,
  PREVIEW_IFRAME_SANDBOX,
  probePreviewReachable,
  validatePreviewUrl,
} from "../lib/webPreviewUrl";
import { Tooltip } from "./Tooltip";

export interface BrowserPanelProps {
  expanded?: boolean;
  onToggleExpanded?: () => void;
  /** Controlled preview URL from workbench state. */
  previewUrl?: string | null;
  onPreviewUrlChange?: (url: string) => void;
  /** Bump to soft-reload iframe after workspace file changes (HMR fallback). */
  refreshKey?: number;
  workspaceRoot?: string;
}

type Reachability = "unknown" | "online" | "offline";

export function BrowserPanel({
  expanded = false,
  onToggleExpanded,
  previewUrl,
  onPreviewUrlChange,
  refreshKey = 0,
  workspaceRoot: _workspaceRoot,
}: BrowserPanelProps) {
  const t = useT();
  const initialUrl = previewUrl?.trim() || defaultPreviewUrl();
  const [url, setUrl] = useState(initialUrl);
  const [src, setSrc] = useState<string | null>(initialUrl);
  const [reachability, setReachability] = useState<Reachability>("unknown");
  const [status, setStatus] = useState<PreviewURLValidation>(() => ({
    decision: "allow",
    url: initialUrl,
    strict: false,
  }));
  const [blockedMessage, setBlockedMessage] = useState("");
  const [iframeLoaded, setIframeLoaded] = useState(false);
  const probeGen = useRef(0);
  const reloadGen = useRef(0);
  const lastRefreshKey = useRef(refreshKey);

  const bumpIframe = useCallback((base: string) => {
    const clean = base.split("#")[0] ?? base;
    setSrc(`${clean}${clean.includes("?") ? "&" : "?"}_preview=${Date.now()}`);
    setIframeLoaded(false);
  }, []);

  const checkReachability = useCallback(async (href: string) => {
    const gen = ++probeGen.current;
    setReachability("unknown");
    const ok = await probePreviewReachable(href);
    if (gen !== probeGen.current) return;
    setReachability(ok ? "online" : "offline");
  }, []);

  const applyPreviewUrl = useCallback(
    async (raw: string, skipConfirm = false) => {
      setBlockedMessage("");
      const next = await (async (): Promise<PreviewURLValidation> => {
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
      })();

      setStatus(next);
      if (next.decision === "blocked") {
        setSrc(null);
        setReachability("offline");
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
        const ok = await confirmAction({
          title: t("browser.externalConfirmTitle"),
          message: t("browser.externalConfirm", { url: next.url }),
        });
        if (!ok) return;
      }

      setUrl(next.url);
      onPreviewUrlChange?.(next.url);
      setSrc(next.url);
      setIframeLoaded(false);
      void checkReachability(next.url);
    },
    [checkReachability, onPreviewUrlChange, t],
  );

  useEffect(() => {
    if (!previewUrl?.trim()) return;
    if (previewUrl.trim() === url.trim()) return;
    void applyPreviewUrl(previewUrl, true);
  }, [applyPreviewUrl, previewUrl, url]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const detected = await app.DetectDevServerURL();
        if (cancelled || !detected) return;
        if (previewUrl?.trim()) return;
        await applyPreviewUrl(detected, true);
      } catch {
        /* ignore */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [applyPreviewUrl, previewUrl, refreshKey]);

  useEffect(() => {
    if (!src || reachability !== "offline") return;
    const id = window.setInterval(() => {
      void checkReachability(url);
    }, 5000);
    return () => window.clearInterval(id);
  }, [checkReachability, reachability, src, url]);

  useEffect(() => {
    if (!src) return;
    if (lastRefreshKey.current === refreshKey) return;
    lastRefreshKey.current = refreshKey;
    const gen = ++reloadGen.current;
    const id = window.setTimeout(() => {
      if (gen !== reloadGen.current) return;
      bumpIframe(url);
    }, 700);
    return () => window.clearTimeout(id);
  }, [bumpIframe, refreshKey, src, url]);

  const reload = useCallback(() => {
    if (!src) {
      void applyPreviewUrl(url, true);
      return;
    }
    bumpIframe(url);
    void checkReachability(url);
  }, [applyPreviewUrl, bumpIframe, checkReachability, src, url]);

  const statusBadge = (() => {
    if (reachability === "offline" && src) {
      return (
        <span className="browser-panel__badge browser-panel__badge--warn">
          <AlertTriangle size={12} />
          {t("browser.offlineBadge")}
        </span>
      );
    }
    if (status.strict) {
      return (
        <span className="browser-panel__badge browser-panel__badge--warn">
          <ShieldCheck size={12} />
          {t("browser.sandboxStrict")}
        </span>
      );
    }
    if (status.decision === "allow" && isLocalUrl(status.url)) {
      return (
        <span className="browser-panel__badge browser-panel__badge--safe">
          <ShieldCheck size={12} />
          {t("browser.sandboxLocal")}
        </span>
      );
    }
    if (status.decision === "confirm") {
      return (
        <span className="browser-panel__badge browser-panel__badge--warn">
          <AlertTriangle size={12} />
          {t("browser.sandboxExternal")}
        </span>
      );
    }
    return (
      <span className="browser-panel__badge">
        <Monitor size={12} />
        {t("browser.sandboxHint")}
      </span>
    );
  })();

  return (
    <div className="right-dock__browser browser-panel">
      <header className="browser-panel__chrome wails-no-drag">
        <div className="browser-panel__row">
          <div className="browser-panel__tools" role="toolbar" aria-label={t("browser.title")}>
            <Tooltip label={t("browser.reload")}>
              <button type="button" className="browser-panel__tool" onClick={reload} aria-label={t("browser.reload")}>
                <RefreshCw size={15} strokeWidth={1.75} />
              </button>
            </Tooltip>
            {onToggleExpanded ? (
              <Tooltip label={t(expanded ? "browser.collapse" : "browser.expand")}>
                <button
                  type="button"
                  className="browser-panel__tool"
                  onClick={onToggleExpanded}
                  aria-label={t(expanded ? "browser.collapse" : "browser.expand")}
                  aria-pressed={expanded}
                >
                  {expanded ? <Minimize2 size={15} strokeWidth={1.75} /> : <Maximize2 size={15} strokeWidth={1.75} />}
                </button>
              </Tooltip>
            ) : null}
          </div>

          <div className="browser-panel__omnibox">
            <input
              className="browser-panel__url"
              value={url}
              onChange={(event) => setUrl(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void applyPreviewUrl(url);
              }}
              placeholder={t("rightDock.urlPlaceholder")}
              spellCheck={false}
              aria-label={t("rightDock.urlPlaceholder")}
            />
            <Tooltip label={t("rightDock.browserGo")}>
              <button
                type="button"
                className="browser-panel__go"
                onClick={() => void applyPreviewUrl(url)}
                aria-label={t("rightDock.browserGo")}
              >
                <ArrowRight size={14} strokeWidth={2} />
              </button>
            </Tooltip>
          </div>
        </div>

        <div className="browser-panel__meta">{statusBadge}</div>
      </header>

      {blockedMessage ? (
        <div className="right-dock__browser-blocked">{blockedMessage}</div>
      ) : src ? (
        <div className="browser-panel__viewport">
          {reachability === "offline" && !iframeLoaded ? (
            <div className="browser-panel__overlay">
              <p>{t("browser.offlineTitle")}</p>
              <p className="browser-panel__idle-hint">{t("browser.offlineHint")}</p>
              <button type="button" className="browser-panel__retry" onClick={() => void applyPreviewUrl(url, true)}>
                {t("browser.retry")}
              </button>
            </div>
          ) : null}
          <iframe
            key={src}
            title={t("browser.title")}
            src={src}
            className="right-dock__iframe"
            sandbox={PREVIEW_IFRAME_SANDBOX}
            referrerPolicy="no-referrer"
            onLoad={() => {
              setIframeLoaded(true);
              if (reachability === "unknown") setReachability("online");
            }}
          />
        </div>
      ) : (
        <div className="right-dock__browser-blocked browser-panel__idle">
          <p>{t("browser.idleTitle")}</p>
          <p className="browser-panel__idle-hint">{t("browser.idleHint")}</p>
        </div>
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
