import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertTriangle,
  ArrowRight,
  Maximize2,
  Minimize2,
  Monitor,
  Plus,
  RefreshCw,
  Scan,
  ShieldCheck,
  X,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import { app } from "../lib/bridge";
import {
  clampPreviewScale,
  computePreviewFitScale,
  DEFAULT_PREVIEW_LOGICAL_HEIGHT,
  DEFAULT_PREVIEW_LOGICAL_WIDTH,
  measureIframeDocumentSize,
  type BrowserPreviewZoomMode,
} from "../lib/browserPreviewFit";
import { browserTabTitle } from "../lib/browserTabTitle";
import { confirmAction } from "../lib/confirmAction";
import { useT } from "../lib/i18n";
import type { BrowserTab } from "../lib/useBrowserPanel";
import type { PreviewURLValidation } from "../lib/types";
import {
  defaultPreviewUrl,
  PREVIEW_IFRAME_SANDBOX,
  probePreviewReachable,
  validatePreviewUrl,
} from "../lib/webPreviewUrl";
import { Tooltip } from "./Tooltip";

export interface BrowserPanelProps {
  tabs: BrowserTab[];
  activeId: string | null;
  onActiveChange: (id: string) => void;
  onCloseTab: (id: string) => void;
  onNewTab: () => void;
  onTabUrlChange: (id: string, url: string, title?: string) => void;
  embedded?: boolean;
  expanded?: boolean;
  onToggleExpanded?: () => void;
  refreshKey?: number;
  workspaceRoot?: string;
}

type Reachability = "unknown" | "online" | "offline";

function isLocalUrl(raw: string): boolean {
  try {
    const host = new URL(raw).hostname.toLowerCase();
    return host === "localhost" || host === "127.0.0.1" || host === "[::1]" || host === "::1";
  } catch {
    return false;
  }
}

function BrowserTabPane({
  tab,
  active,
  refreshKey,
  onUrlChange,
}: {
  tab: BrowserTab;
  active: boolean;
  refreshKey: number;
  onUrlChange: (url: string, title: string) => void;
}) {
  const t = useT();
  const [addressDraft, setAddressDraft] = useState(tab.url);
  const [src, setSrc] = useState<string | null>(tab.url);
  const [reachability, setReachability] = useState<Reachability>("unknown");
  const [status, setStatus] = useState<PreviewURLValidation>(() => ({
    decision: "allow",
    url: tab.url,
    strict: false,
  }));
  const [blockedMessage, setBlockedMessage] = useState("");
  const [iframeLoaded, setIframeLoaded] = useState(false);
  const [zoomMode, setZoomMode] = useState<BrowserPreviewZoomMode>("fit");
  const [manualScale, setManualScale] = useState(1);
  const [viewportSize, setViewportSize] = useState({ width: 0, height: 0 });
  const [contentSize, setContentSize] = useState({
    width: DEFAULT_PREVIEW_LOGICAL_WIDTH,
    height: DEFAULT_PREVIEW_LOGICAL_HEIGHT,
  });
  const viewportRef = useRef<HTMLDivElement>(null);
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const probeGen = useRef(0);
  const reloadGen = useRef(0);
  const lastRefreshKey = useRef(refreshKey);
  const lastTabIdRef = useRef(tab.id);
  const prevTabUrlRef = useRef(tab.url);

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

      prevTabUrlRef.current = next.url;
      setAddressDraft(next.url);
      onUrlChange(next.url, browserTabTitle(next.url));
      setSrc(next.url);
      setIframeLoaded(false);
      setZoomMode("fit");
      void checkReachability(next.url);
    },
    [checkReachability, onUrlChange, t],
  );

  useEffect(() => {
    if (lastTabIdRef.current !== tab.id) {
      lastTabIdRef.current = tab.id;
      prevTabUrlRef.current = tab.url;
      setAddressDraft(tab.url);
      setZoomMode("fit");
      if (tab.url.trim()) {
        setSrc(tab.url);
      }
      return;
    }
    if (prevTabUrlRef.current === tab.url) return;
    prevTabUrlRef.current = tab.url;
    setAddressDraft(tab.url);
    if (tab.url.trim()) {
      void applyPreviewUrl(tab.url, true);
    }
  }, [applyPreviewUrl, tab.id, tab.url]);

  useEffect(() => {
    if (!active || tab.url.trim()) return;
    let cancelled = false;
    void (async () => {
      try {
        const detected = await app.DetectDevServerURL();
        if (cancelled || !detected) return;
        await applyPreviewUrl(detected, true);
      } catch {
        /* ignore */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [active, applyPreviewUrl, tab.url]);

  useEffect(() => {
    if (!src || reachability !== "offline") return;
    const id = window.setInterval(() => {
      void checkReachability(addressDraft);
    }, 5000);
    return () => window.clearInterval(id);
  }, [addressDraft, checkReachability, reachability, src]);

  useEffect(() => {
    if (!active || !src) return;
    if (lastRefreshKey.current === refreshKey) return;
    lastRefreshKey.current = refreshKey;
    const gen = ++reloadGen.current;
    const id = window.setTimeout(() => {
      if (gen !== reloadGen.current) return;
      bumpIframe(addressDraft);
    }, 700);
    return () => window.clearTimeout(id);
  }, [active, addressDraft, bumpIframe, refreshKey, src]);

  const measureContent = useCallback(() => {
    const iframe = iframeRef.current;
    if (!iframe) return;
    const measured = measureIframeDocumentSize(iframe);
    if (measured) {
      setContentSize(measured);
    }
  }, []);

  useEffect(() => {
    const el = viewportRef.current;
    if (!el || !active) return;
    const update = () => {
      const rect = el.getBoundingClientRect();
      setViewportSize({ width: rect.width, height: rect.height });
    };
    update();
    const ro = new ResizeObserver(() => update());
    ro.observe(el);
    return () => ro.disconnect();
  }, [active]);

  const previewScale = useMemo(() => {
    if (zoomMode === "manual") return clampPreviewScale(manualScale);
    return clampPreviewScale(
      computePreviewFitScale(
        viewportSize.width,
        viewportSize.height,
        contentSize.width,
        contentSize.height,
      ),
    );
  }, [contentSize.height, contentSize.width, manualScale, viewportSize.height, viewportSize.width, zoomMode]);

  const zoomIn = useCallback(() => {
    setZoomMode("manual");
    setManualScale((current) => clampPreviewScale((zoomMode === "manual" ? current : previewScale) * 1.12));
  }, [previewScale, zoomMode]);

  const zoomOut = useCallback(() => {
    setZoomMode("manual");
    setManualScale((current) => clampPreviewScale((zoomMode === "manual" ? current : previewScale) / 1.12));
  }, [previewScale, zoomMode]);

  const zoomFit = useCallback(() => {
    setZoomMode("fit");
    measureContent();
  }, [measureContent]);

  const reload = useCallback(() => {
    if (!src) {
      void applyPreviewUrl(addressDraft, true);
      return;
    }
    bumpIframe(addressDraft);
    void checkReachability(addressDraft);
  }, [addressDraft, applyPreviewUrl, bumpIframe, checkReachability, src]);

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
    <div className={`browser-panel__pane${active ? " browser-panel__pane--active" : ""}`} aria-hidden={!active}>
      <header className="browser-panel__chrome wails-no-drag">
        <div className="browser-panel__row">
          <div className="browser-panel__tools" role="toolbar" aria-label={t("browser.title")}>
            <Tooltip label={t("browser.reload")}>
              <button type="button" className="browser-panel__tool" onClick={reload} aria-label={t("browser.reload")}>
                <RefreshCw size={15} strokeWidth={1.75} />
              </button>
            </Tooltip>
            <Tooltip label={t("browser.zoomFit")}>
              <button
                type="button"
                className={`browser-panel__tool${zoomMode === "fit" ? " browser-panel__tool--active" : ""}`}
                onClick={zoomFit}
                aria-label={t("browser.zoomFit")}
                aria-pressed={zoomMode === "fit"}
              >
                <Scan size={15} strokeWidth={1.75} />
              </button>
            </Tooltip>
            <Tooltip label={t("browser.zoomOut")}>
              <button type="button" className="browser-panel__tool" onClick={zoomOut} aria-label={t("browser.zoomOut")}>
                <ZoomOut size={15} strokeWidth={1.75} />
              </button>
            </Tooltip>
            <span className="browser-panel__zoom-level" aria-live="polite">
              {Math.round(previewScale * 100)}%
            </span>
            <Tooltip label={t("browser.zoomIn")}>
              <button type="button" className="browser-panel__tool" onClick={zoomIn} aria-label={t("browser.zoomIn")}>
                <ZoomIn size={15} strokeWidth={1.75} />
              </button>
            </Tooltip>
          </div>
          <div className="browser-panel__omnibox">
            <input
              className="browser-panel__url wails-no-drag"
              value={addressDraft}
              onChange={(event) => setAddressDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void applyPreviewUrl(addressDraft);
              }}
              placeholder={t("rightDock.urlPlaceholder")}
              spellCheck={false}
              aria-label={t("rightDock.urlPlaceholder")}
            />
            <Tooltip label={t("rightDock.browserGo")}>
              <button
                type="button"
                className="browser-panel__go"
                onClick={() => void applyPreviewUrl(addressDraft)}
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
        <div
          ref={viewportRef}
          className={`browser-panel__viewport${zoomMode === "manual" ? " browser-panel__viewport--manual" : ""}`}
        >
          {reachability === "offline" && !iframeLoaded ? (
            <div className="browser-panel__overlay">
              <p>{t("browser.offlineTitle")}</p>
              <p className="browser-panel__idle-hint">{t("browser.offlineHint")}</p>
              <button type="button" className="browser-panel__retry" onClick={() => void applyPreviewUrl(addressDraft, true)}>
                {t("browser.retry")}
              </button>
            </div>
          ) : null}
          <div
            className="browser-panel__scaler"
            style={{
              width: contentSize.width,
              height: contentSize.height,
              transform: `scale(${previewScale})`,
            }}
          >
            <iframe
              ref={iframeRef}
              key={src}
              title={tab.title}
              src={src}
              className="right-dock__iframe"
              sandbox={PREVIEW_IFRAME_SANDBOX}
              referrerPolicy="no-referrer"
              onLoad={() => {
                setIframeLoaded(true);
                if (reachability === "unknown") setReachability("online");
                measureContent();
                requestAnimationFrame(() => measureContent());
                window.setTimeout(() => measureContent(), 400);
              }}
            />
          </div>
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

export function BrowserPanel({
  tabs,
  activeId,
  onActiveChange,
  onCloseTab,
  onNewTab,
  onTabUrlChange,
  embedded = false,
  expanded = false,
  onToggleExpanded,
  refreshKey = 0,
}: BrowserPanelProps) {
  const t = useT();
  const tabsRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = tabsRef.current;
    if (!el || !activeId) return;
    const activeBtn = el.querySelector<HTMLButtonElement>(`[data-browser-id="${activeId}"]`);
    activeBtn?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [activeId, tabs.length]);

  if (tabs.length === 0) {
    return (
      <div className={`right-dock__browser browser-panel browser-panel--empty${embedded ? " browser-panel--embedded" : ""}`}>
        <p>{t("browser.idleTitle")}</p>
        <button type="button" className="browser-panel__retry" onClick={onNewTab}>
          {t("browser.newTab")}
        </button>
      </div>
    );
  }

  return (
    <div className={`right-dock__browser browser-panel${embedded ? " browser-panel--embedded" : ""}`}>
      <header className="browser-panel__tabbar wails-no-drag">
        <div className="browser-panel__tabs" ref={tabsRef} role="tablist" aria-label={t("browser.tabs")}>
          {tabs.map((tab) => (
            <div
              key={tab.clientKey}
              className={`browser-panel__tab${tab.id === activeId ? " browser-panel__tab--active" : ""}`}
              role="presentation"
            >
              <button
                type="button"
                role="tab"
                data-browser-id={tab.id}
                aria-selected={tab.id === activeId}
                className="browser-panel__tab-main"
                onClick={() => onActiveChange(tab.id)}
              >
                <Monitor size={12} />
                <span>{tab.title}</span>
              </button>
              <button
                type="button"
                className="browser-panel__tab-close"
                aria-label={t("browser.closeTab", { title: tab.title })}
                onClick={(event) => {
                  event.stopPropagation();
                  onCloseTab(tab.id);
                }}
              >
                <X size={12} />
              </button>
            </div>
          ))}
        </div>
        <div className="browser-panel__tab-actions">
          <button type="button" className="browser-panel__action" onClick={onNewTab} aria-label={t("browser.newTab")}>
            <Plus size={14} />
          </button>
          {!embedded && onToggleExpanded ? (
            <Tooltip label={t(expanded ? "browser.collapse" : "browser.expand")}>
              <button
                type="button"
                className="browser-panel__action"
                onClick={onToggleExpanded}
                aria-label={t(expanded ? "browser.collapse" : "browser.expand")}
                aria-pressed={expanded}
              >
                {expanded ? <Minimize2 size={14} /> : <Maximize2 size={14} />}
              </button>
            </Tooltip>
          ) : null}
        </div>
      </header>
      <div className="browser-panel__stack">
        {tabs.map((tab) => (
          <BrowserTabPane
            key={tab.clientKey}
            tab={tab}
            active={tab.id === activeId}
            refreshKey={refreshKey}
            onUrlChange={(url, title) => onTabUrlChange(tab.id, url, title ?? browserTabTitle(url))}
          />
        ))}
      </div>
    </div>
  );
}

export { defaultPreviewUrl };
