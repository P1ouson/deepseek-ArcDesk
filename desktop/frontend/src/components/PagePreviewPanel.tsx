import { useCallback, useEffect, useRef, useState } from "react";
import { AlertTriangle, Maximize2, Minimize2, RefreshCw } from "lucide-react";
import { app } from "../lib/bridge";
import { toErrorMessage } from "../lib/errors";
import { useT } from "../lib/i18n";
import { PREVIEW_IFRAME_SANDBOX } from "../lib/webPreviewUrl";
import { basename, isPreviewablePagePath } from "../lib/previewPage";
import { Tooltip } from "./Tooltip";

export interface PagePreviewPanelProps {
  embedded?: boolean;
  expanded?: boolean;
  onToggleExpanded?: () => void;
  pagePath?: string | null;
  onPagePathChange?: (path: string) => void;
  refreshKey?: number;
  workspaceRoot?: string;
}

export function PagePreviewPanel({
  embedded = false,
  expanded = false,
  onToggleExpanded,
  pagePath,
  onPagePathChange,
  refreshKey = 0,
}: PagePreviewPanelProps) {
  const t = useT();
  const [src, setSrc] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const lastPath = useRef<string | null>(null);
  const lastRefreshKey = useRef(refreshKey);
  const reloadGen = useRef(0);

  const loadPage = useCallback(
    async (rel: string, bump = false) => {
      const trimmed = rel.trim();
      if (!trimmed) {
        setSrc(null);
        setError("");
        return;
      }
      if (!isPreviewablePagePath(trimmed)) {
        setSrc(null);
        setError(t("pagePreview.unsupported"));
        return;
      }
      setLoading(true);
      setError("");
      try {
        const url = await app.WorkspacePagePreviewURL(trimmed);
        const next = bump ? `${url}${url.includes("?") ? "&" : "?"}_t=${Date.now()}` : url;
        setSrc(next);
        lastPath.current = trimmed;
        onPagePathChange?.(trimmed);
      } catch (err) {
        setSrc(null);
        setError(toErrorMessage(err, t("pagePreview.loadFailed")));
      } finally {
        setLoading(false);
      }
    },
    [onPagePathChange, t],
  );

  useEffect(() => {
    if (!pagePath?.trim()) {
      setSrc(null);
      setError("");
      return;
    }
    if (pagePath.trim() === lastPath.current && src) return;
    void loadPage(pagePath, false);
  }, [loadPage, pagePath, src]);

  useEffect(() => {
    if (!pagePath?.trim() || !src) return;
    if (lastRefreshKey.current === refreshKey) return;
    lastRefreshKey.current = refreshKey;
    const gen = ++reloadGen.current;
    const id = window.setTimeout(() => {
      if (gen !== reloadGen.current) return;
      void loadPage(pagePath, true);
    }, 500);
    return () => window.clearTimeout(id);
  }, [loadPage, pagePath, refreshKey, src]);

  const reload = useCallback(() => {
    if (!pagePath?.trim()) return;
    void loadPage(pagePath, true);
  }, [loadPage, pagePath]);

  const title = pagePath ? basename(pagePath) : t("pagePreview.title");

  return (
    <div className={`right-dock__browser browser-panel page-preview-panel${embedded ? " page-preview-panel--embedded" : ""}`}>
      <header className="browser-panel__chrome wails-no-drag">
        <div className="browser-panel__row">
          <div className="browser-panel__tools" role="toolbar" aria-label={t("pagePreview.title")}>
            <Tooltip label={t("pagePreview.reload")}>
              <button
                type="button"
                className="browser-panel__tool"
                onClick={reload}
                disabled={!pagePath?.trim()}
                aria-label={t("pagePreview.reload")}
              >
                <RefreshCw size={15} strokeWidth={1.75} />
              </button>
            </Tooltip>
            {!embedded && onToggleExpanded ? (
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
          <div className="browser-panel__omnibox page-preview-panel__path">
            <span className="page-preview-panel__path-text" title={pagePath ?? undefined}>
              {title}
            </span>
          </div>
        </div>
      </header>

      {error ? (
        <div className="right-dock__browser-blocked page-preview-panel__empty">
          <AlertTriangle size={18} />
          <p>{error}</p>
        </div>
      ) : src ? (
        <div className="browser-panel__viewport">
          {loading ? <div className="page-preview-panel__loading">{t("pagePreview.loading")}</div> : null}
          <iframe
            key={src}
            title={t("pagePreview.title")}
            src={src}
            className="right-dock__iframe"
            sandbox={PREVIEW_IFRAME_SANDBOX}
            referrerPolicy="no-referrer"
            onLoad={() => setLoading(false)}
          />
        </div>
      ) : (
        <div className="right-dock__browser-blocked browser-panel__idle page-preview-panel__empty">
          <p>{t("pagePreview.idleTitle")}</p>
          <p className="browser-panel__idle-hint">{t("pagePreview.idleHint")}</p>
        </div>
      )}
    </div>
  );
}
