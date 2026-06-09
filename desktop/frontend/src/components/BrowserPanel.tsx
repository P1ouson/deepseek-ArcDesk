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
}

type LoadPhase = "idle" | "probing" | "ready" | "offline";

export function BrowserPanel({ expanded = false, onToggleExpanded }: BrowserPanelProps) {
  const t = useT();
  const initialUrl = defaultPreviewUrl();
  const [url, setUrl] = useState(initialUrl);
  const [src, setSrc] = useState<string | null>(null);
  const [phase, setPhase] = useState<LoadPhase>("idle");
  const [status, setStatus] = useState<PreviewURLValidation>(() => ({
    decision: "allow",
    url: initialUrl,
    strict: false,
  }));
  const [blockedMessage, setBlockedMessage] = useState("");
  const probeGen = useRef(0);

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

  const loadPreview = useCallback(
    async (raw: string, skipConfirm = false) => {
      const gen = ++probeGen.current;
      setBlockedMessage("");
      setPhase("probing");
      setSrc(null);

      const next = await resolveUrl(raw);
      if (gen !== probeGen.current) return;

      setStatus(next);
      if (next.decision === "blocked") {
        setPhase("idle");
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
        if (!ok) {
          setPhase("idle");
          return;
        }
      }

      setUrl(next.url);
      const reachable = await probePreviewReachable(next.url);
      if (gen !== probeGen.current) return;

      if (!reachable) {
        setPhase("offline");
        setSrc(null);
        return;
      }

      setSrc(next.url);
      setPhase("ready");
    },
    [resolveUrl, t],
  );

  useEffect(() => {
    void loadPreview(initialUrl, true);
  }, [initialUrl, loadPreview]);

  useEffect(() => {
    if (phase !== "offline") return;
    const id = window.setInterval(() => {
      void loadPreview(url, true);
    }, 5000);
    return () => window.clearInterval(id);
  }, [loadPreview, phase, url]);

  const reload = useCallback(() => {
    if (!src) {
      void loadPreview(url, true);
      return;
    }
    const base = src.split("#")[0] ?? src;
    setSrc(`${base}${base.includes("?") ? "&" : "?"}_preview=${Date.now()}`);
  }, [loadPreview, src, url]);

  const statusBadge = (() => {
    if (phase === "offline") {
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
              <button
                type="button"
                className="browser-panel__tool"
                onClick={reload}
                disabled={phase === "probing"}
                aria-label={t("browser.reload")}
              >
                <RefreshCw size={15} strokeWidth={1.75} className={phase === "probing" ? "dock-panel__spin" : undefined} />
              </button>
            </Tooltip>
            {onToggleExpanded && (
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
            )}
          </div>

          <div className="browser-panel__omnibox">
            <input
              className="browser-panel__url"
              value={url}
              onChange={(event) => setUrl(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void loadPreview(url);
              }}
              placeholder={t("rightDock.urlPlaceholder")}
              spellCheck={false}
              aria-label={t("rightDock.urlPlaceholder")}
            />
            <Tooltip label={t("rightDock.browserGo")}>
              <button
                type="button"
                className="browser-panel__go"
                onClick={() => void loadPreview(url)}
                disabled={phase === "probing"}
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
      ) : phase === "probing" ? (
        <div className="right-dock__browser-blocked browser-panel__idle">
          <p>{t("browser.probing")}</p>
        </div>
      ) : phase === "offline" ? (
        <div className="right-dock__browser-blocked browser-panel__idle">
          <p>{t("browser.offlineTitle")}</p>
          <p className="browser-panel__idle-hint">{t("browser.offlineHint")}</p>
          <button type="button" className="browser-panel__retry" onClick={() => void loadPreview(url, true)}>
            {t("browser.retry")}
          </button>
        </div>
      ) : src ? (
        <iframe
          key={src}
          title={t("browser.title")}
          src={src}
          className="right-dock__iframe"
          sandbox={PREVIEW_IFRAME_SANDBOX}
          referrerPolicy="no-referrer"
        />
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
