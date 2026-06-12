import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  Cloud,
  Copy,
  Eye,
  EyeOff,
  Globe,
  Power,
  QrCode,
  RefreshCw,
  Smartphone,
  Wifi,
} from "lucide-react";
import { asArray } from "../lib/array";
import { app } from "../lib/bridge";
import { toErrorMessage } from "../lib/errors";
import { useT } from "../lib/i18n";
import type { MobileConnectConfig, MobilePairingInfo, MobilePendingDecision, MobileTunnelStatus, ModelInfo } from "../lib/types";

function formatExpiry(ts: number, t: ReturnType<typeof useT>): string {
  if (!ts) return "—";
  const mins = Math.max(1, Math.round(Math.max(0, ts - Date.now()) / 60_000));
  return t("phone.qrExpires", { n: mins });
}

function modeMeta(mode: string | undefined, t: ReturnType<typeof useT>) {
  switch (mode) {
    case "tunnel":
      return { label: t("phone.modeTunnel"), icon: Cloud, ok: true };
    case "relay":
      return { label: t("phone.modeRelay"), icon: Cloud, ok: true };
    case "lan":
      return { label: t("phone.modeLan"), icon: Wifi, ok: true };
    case "lan_standby":
      return { label: t("phone.modeLanStandby"), icon: Wifi, ok: false };
    default:
      return { label: t("phone.modeNone"), icon: Globe, ok: false };
  }
}

function basename(path: string): string {
  const normalized = path.replace(/\\/g, "/").replace(/\/+$/, "");
  const parts = normalized.split("/").filter(Boolean);
  return parts[parts.length - 1] || path || "—";
}

function maskPairUrl(url: string): string {
  try {
    const parsed = new URL(url);
    const host = parsed.host;
    const maskedHost =
      host.length <= 6 ? "••••••" : `${host.slice(0, 3)}••••${host.slice(-3)}`;
    return `${parsed.protocol}//${maskedHost}${parsed.pathname}`;
  } catch {
    return "••••••••";
  }
}

function ProtectedPairUrl({
  url,
  revealed,
  onToggleReveal,
  onCopy,
  copied,
  copyLabel,
}: {
  url: string;
  revealed: boolean;
  onToggleReveal: () => void;
  onCopy: () => void;
  copied: boolean;
  copyLabel: string;
}) {
  return (
    <div className="connect-compact__url-protected">
      <code className="connect-compact__url">{revealed ? url : maskPairUrl(url)}</code>
      <div className="connect-compact__url-actions">
        <button
          type="button"
          className="connect-compact__url-action"
          onClick={onToggleReveal}
          aria-label={revealed ? "Hide URL" : "Show URL"}
        >
          {revealed ? <EyeOff size={12} /> : <Eye size={12} />}
        </button>
        <button
          type="button"
          className={`connect-compact__url-action${copied ? " connect-compact__url-action--ok" : ""}`}
          onClick={onCopy}
          aria-label={copyLabel}
        >
          {copied ? <Check size={12} /> : <Copy size={12} />}
        </button>
      </div>
    </div>
  );
}

export interface ConnectPhoneViewProps {
  workspaceRoot: string;
  activeTabId?: string;
  tabLabel?: string;
  workspaceName?: string;
}

export function ConnectPhoneView({
  workspaceRoot,
  activeTabId,
  tabLabel,
  workspaceName,
}: ConnectPhoneViewProps) {
  const t = useT();
  const [pairing, setPairing] = useState<MobilePairingInfo | null>(null);
  const [tunnel, setTunnel] = useState<MobileTunnelStatus>({ running: false, url: "" });
  const [sessions, setSessions] = useState<{ id: string; createdAt: number; lastSeen: number }[]>([]);
  const [currentModel, setCurrentModel] = useState("");
  const [config, setConfig] = useState<MobileConnectConfig>(() => ({
    enabled: true,
    allowLAN: false,
    model: "",
    persona: "",
    workspaceRoot,
    relayBaseURL: "",
  }));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [pendingDecision, setPendingDecision] = useState<MobilePendingDecision | null>(null);
  const [copiedField, setCopiedField] = useState<"pair" | "tunnel" | null>(null);
  const [pairUrlRevealed, setPairUrlRevealed] = useState(false);
  const copyTimerRef = useRef<number | null>(null);

  const reload = useCallback(async () => {
    const [info, cfg, tunnelStatus, deviceList, models] = await Promise.all([
      app.GetMobilePairingInfo().catch(() => null),
      app.GetMobileConnectConfig().catch(() => null),
      app.GetMobileTunnelStatus().catch(() => ({ running: false, url: "" })),
      app.ListMobileSessions().catch(() => []),
      (activeTabId ? app.ModelsForTab(activeTabId) : app.Models()).catch((): ModelInfo[] => []),
    ]);
    if (info) setPairing(info);
    if (cfg) {
      setConfig({
        ...cfg,
        allowLAN: Boolean(cfg.allowLAN),
        workspaceRoot,
        relayBaseURL: cfg.relayBaseURL ?? "",
      });
    }
    const active = asArray(models).find((m: ModelInfo) => m.current);
    setCurrentModel(active?.ref ?? active?.model ?? "");
    setTunnel(tunnelStatus);
    setSessions(deviceList);
  }, [activeTabId, workspaceRoot]);

  useEffect(() => {
    void reload();
    const timer = window.setInterval(() => void reload(), 2_000);
    return () => window.clearInterval(timer);
  }, [reload]);

  useEffect(() => {
    if (!tunnel.running || tunnel.url || tunnel.err) return;
    const timer = window.setInterval(() => void reload(), tunnel.phase === "downloading" ? 500 : 1_000);
    return () => window.clearInterval(timer);
  }, [tunnel.running, tunnel.url, tunnel.err, tunnel.phase, reload]);

  useEffect(() => {
    const offMessage = window.runtime?.EventsOn?.("mobile:message", () => void reload());
    const offSessions = window.runtime?.EventsOn?.("mobile:sessions", () => void reload());
    return () => {
      offMessage?.();
      offSessions?.();
    };
  }, [reload]);

  const mode = useMemo(() => modeMeta(pairing?.connectMode, t), [pairing?.connectMode, t]);
  const ModeIcon = mode.icon;
  const tunnelReady = Boolean(tunnel.url || pairing?.tunnelUrl);
  const lanTransportReady = Boolean(config.allowLAN && pairing?.lanIp);
  const remoteTransportReady = tunnelReady || lanTransportReady;
  const lanAddress = pairing?.lanIp ? `${pairing.lanIp}:${pairing.port}` : t("phone.lanNotDetected");
  const isLanMode = pairing?.connectMode === "lan";
  const activeConnectionCount = remoteTransportReady
    ? (pairing?.activeCount ?? tunnel.activeCount ?? 0)
    : 0;
  const qrHint = useMemo(() => {
    if (pairing?.bridgeReady === false) return t("phone.bridgeUnavailable");
    if (pairing?.qrDataUrl || pairing?.pairUrl) return "";
    if (tunnelReady || pairing?.connectMode === "tunnel") return "";
    if (config.allowLAN && pairing?.lanIp) return t("phone.qrLanRefresh");
    if (pairing?.lanIp && !config.allowLAN) return t("phone.qrEnableLan", { ip: pairing.lanIp });
    return t("phone.qrStartTunnel");
  }, [config.allowLAN, pairing?.bridgeReady, pairing?.connectMode, pairing?.lanIp, pairing?.pairUrl, pairing?.qrDataUrl, t, tunnelReady]);

  useEffect(() => {
    if (tunnelReady) {
      setNotice((prev) => (prev === t("phone.tunnelStarting") ? null : prev));
    }
  }, [tunnelReady, t]);

  const tunnelPhaseHint = useMemo(() => {
    if (tunnel.err || tunnel.phase === "downloading") return null;
    if (!tunnel.running || tunnel.url) return null;
    return t("phone.tunnelWaiting");
  }, [tunnel.err, tunnel.phase, tunnel.running, tunnel.url, t]);
  const downloadProgressLabel = useMemo(() => {
    if (tunnel.phase !== "downloading") return "";
    const pct = tunnel.downloadProgress;
    if (pct != null && pct >= 0 && pct <= 100) return `${t("phone.tunnelDownloading")} ${pct}%`;
    return t("phone.tunnelDownloading");
  }, [t, tunnel.downloadProgress, tunnel.phase]);
  const displayWorkspace = workspaceName || basename(workspaceRoot) || workspaceRoot || "—";
  const displayTab = tabLabel?.trim() || "—";

  const copyText = async (text: string, field: "pair" | "tunnel") => {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      setCopiedField(field);
      if (copyTimerRef.current) window.clearTimeout(copyTimerRef.current);
      copyTimerRef.current = window.setTimeout(() => setCopiedField(null), 2000);
    } catch {
      /* clipboard unavailable */
    }
  };

  useEffect(() => {
    return () => {
      if (copyTimerRef.current) window.clearTimeout(copyTimerRef.current);
    };
  }, []);

  useEffect(() => {
    const tick = () => {
      void app.GetMobilePendingDecision().then(setPendingDecision).catch(() => setPendingDecision(null));
    };
    tick();
    const id = window.setInterval(tick, 2500);
    return () => window.clearInterval(id);
  }, []);

  const saveConnectConfig = async (nextConfig: MobileConnectConfig, rollback?: () => void) => {
    setConfig(nextConfig);
    setBusy(true);
    setErr(null);
    try {
      await app.SaveMobileConnectConfig(nextConfig);
      setNotice(t("phone.saved"));
      if (nextConfig.allowLAN) {
        setPairing(await app.RefreshMobilePairing());
      } else {
        await reload();
      }
    } catch (e) {
      rollback?.();
      const raw = toErrorMessage(e);
      const normalized = raw.trim().toLowerCase();
      if (normalized === "cancelled" || normalized.includes("cancelled")) {
        setErr(t("phone.lanEnableCancelled"));
      } else if (normalized.includes("restart claw bridge") || normalized.includes("listen on")) {
        setErr(t("phone.lanEnableFailed"));
      } else {
        setErr(raw);
      }
    } finally {
      setBusy(false);
    }
  };

  const toggleEnabled = async (enabled: boolean) => {
    const prev = config.enabled;
    await saveConnectConfig(
      {
        ...config,
        enabled,
        workspaceRoot,
        relayBaseURL: config.relayBaseURL?.trim() ?? "",
      },
      () => setConfig((v) => ({ ...v, enabled: prev })),
    );
  };

  const toggleAllowLAN = async (allowLAN: boolean) => {
    const prev = Boolean(config.allowLAN);
    await saveConnectConfig(
      {
        ...config,
        allowLAN,
        workspaceRoot,
        relayBaseURL: config.relayBaseURL?.trim() ?? "",
      },
      () => setConfig((v) => ({ ...v, allowLAN: prev })),
    );
  };

  const toggleTunnelIdleShutdown = async (enabled: boolean) => {
    const prev = Boolean(config.tunnelDisableIdleShutdown);
    await saveConnectConfig(
      {
        ...config,
        tunnelDisableIdleShutdown: !enabled,
        workspaceRoot,
        relayBaseURL: config.relayBaseURL?.trim() ?? "",
      },
      () => setConfig((v) => ({ ...v, tunnelDisableIdleShutdown: prev })),
    );
  };

  const refreshQR = async () => {
    setBusy(true);
    setErr(null);
    try {
      setPairing(await app.RefreshMobilePairing());
      setNotice(t("phone.qrRefreshed"));
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const startTunnel = async () => {
    setBusy(true);
    setErr(null);
    setNotice(null);
    try {
      const status = await app.StartMobileTunnel();
      setTunnel(status);
      if (status.err) {
        if (status.err.toLowerCase().includes("cancelled")) {
          setNotice(null);
          return;
        }
        setErr(status.err);
        return;
      }
      setNotice(t("phone.tunnelStarting"));
      await reload();
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const revokeDevice = async (sessionId: string) => {
    setBusy(true);
    setErr(null);
    try {
      await app.RevokeMobileSession(sessionId);
      setSessions(await app.ListMobileSessions().catch(() => []));
      setNotice(t("phone.deviceRevoked"));
      await reload();
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const stopTunnel = async () => {
    setBusy(true);
    setErr(null);
    try {
      setTunnel(await app.StopMobileTunnel());
      await reload();
      setNotice(t("phone.tunnelStopped"));
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="connect-compact">
      <header className="connect-compact__head">
        <div>
          <h2>{t("phone.title")}</h2>
          <p>{t("phone.panelHint")}</p>
        </div>
        <div className="connect-compact__chips">
          <span className={`connect-compact__chip${mode.ok ? " connect-compact__chip--ok" : ""}`}>
            <ModeIcon size={12} /> {mode.label}
          </span>
          <span className={`connect-compact__chip${activeConnectionCount > 0 ? " connect-compact__chip--ok" : ""}`}>
            <Smartphone size={12} /> {t("phone.activeDevices", { n: activeConnectionCount })}
          </span>
          <span className={`connect-compact__chip${tunnelReady ? " connect-compact__chip--ok" : ""}`}>
            <Cloud size={12} /> {tunnelReady ? t("phone.tunnelReadyShort") : tunnel.running ? t("phone.tunnelWaitingShort") : t("phone.tunnelIdle")}
          </span>
          <span className="connect-compact__chip">
            <QrCode size={12} /> {formatExpiry(pairing?.expiresAt ?? 0, t)}
          </span>
        </div>
      </header>

      {err ? <div className="connect-compact__banner connect-compact__banner--error">{err}</div> : null}
      {pairing && pairing.bridgeReady === false ? (
        <div className="connect-compact__banner connect-compact__banner--error">{t("phone.bridgeUnavailable")}</div>
      ) : null}
      {pendingDecision ? (
        <div className="connect-compact__banner connect-compact__banner--decision">
          <div>
            <strong>{t("phone.pendingDecisionTitle")}</strong>
            <p>{pendingDecision.title}</p>
            {pendingDecision.summary ? <p className="connect-compact__decision-summary">{pendingDecision.summary}</p> : null}
          </div>
          {pendingDecision.kind === "approval" ? (
            <div className="connect-compact__decision-actions">
              <button
                type="button"
                disabled={busy}
                onClick={() => {
                  setBusy(true);
                  void app
                    .RespondMobileDecision(pendingDecision.id, true, [])
                    .then(() => setPendingDecision(null))
                    .catch((e) => setErr(toErrorMessage(e)))
                    .finally(() => setBusy(false));
                }}
              >
                {t("phone.pendingDecisionAllow")}
              </button>
            </div>
          ) : null}
        </div>
      ) : null}
      {notice ? <div className="connect-compact__banner connect-compact__banner--ok">{notice}</div> : null}

      <div className="connect-compact__body">
        <section className="connect-compact__qr-panel">
          <div className="connect-compact__qr-head">
            <strong>{t("phone.scanTitle")}</strong>
            <button type="button" className="connect-compact__icon-btn" disabled={busy} onClick={() => void refreshQR()} title={t("phone.refreshQr")}>
              <RefreshCw size={13} />
            </button>
          </div>
          <div className="connect-compact__qr-box">
            {pairing?.qrDataUrl ? (
              <img className="connect-compact__qr" src={pairing.qrDataUrl} alt={t("phone.qrAlt")} />
            ) : (
              <div className="connect-compact__qr-empty">
                <p>{qrHint}</p>
                {!config.allowLAN && !tunnel.running && pairing?.lanIp ? (
                  <button
                    type="button"
                    className="connect-compact__btn connect-compact__btn--primary"
                    disabled={busy}
                    onClick={() => void toggleAllowLAN(true)}
                  >
                    <Wifi size={13} />
                    {t("phone.enableLanPairing")}
                  </button>
                ) : null}
                {!config.allowLAN && !tunnel.running && !pairing?.lanIp ? (
                  <button
                    type="button"
                    className="connect-compact__btn connect-compact__btn--primary"
                    disabled={busy}
                    onClick={() => void startTunnel()}
                  >
                    <Cloud size={13} />
                    {t("phone.tunnelStart")}
                  </button>
                ) : null}
                {config.allowLAN && pairing?.lanIp && !pairing?.qrDataUrl ? (
                  <button
                    type="button"
                    className="connect-compact__btn connect-compact__btn--primary"
                    disabled={busy}
                    onClick={() => void refreshQR()}
                  >
                    <RefreshCw size={13} />
                    {t("phone.refreshQr")}
                  </button>
                ) : null}
              </div>
            )}
          </div>
          {pairing?.pairUrl ? (
            <>
              <ProtectedPairUrl
                url={pairing.pairUrl}
                revealed={pairUrlRevealed}
                onToggleReveal={() => setPairUrlRevealed((v) => !v)}
                onCopy={() => void copyText(pairing.pairUrl, "pair")}
                copied={copiedField === "pair"}
                copyLabel={copiedField === "pair" ? t("phone.copied") : t("common.copy")}
              />
              {isLanMode ? (
                <p className="connect-compact__lan-hint">{t("phone.lanHttpHint")}</p>
              ) : pairing.connectMode === "tunnel" ? (
                <p className="connect-compact__lan-hint">{t("phone.tunnelScanHint")}</p>
              ) : null}
            </>
          ) : null}
          <p className="connect-compact__tunnel-warning">{t("phone.tunnelSecurityWarning")}</p>
          <div className="connect-compact__tunnel">
            {tunnel.phase === "downloading" ? (
              <div className="connect-compact__download-progress">
                <div className="connect-compact__download-progress-track">
                  <div
                    className={`connect-compact__download-progress-bar${tunnel.downloadProgress != null && tunnel.downloadProgress < 0 ? " connect-compact__download-progress-bar--indeterminate" : ""}`}
                    style={
                      tunnel.downloadProgress != null && tunnel.downloadProgress >= 0
                        ? { width: `${Math.max(4, tunnel.downloadProgress)}%` }
                        : undefined
                    }
                  />
                </div>
                <span>{downloadProgressLabel}</span>
              </div>
            ) : null}
            {tunnel.running ? (
              <button type="button" className="connect-compact__btn connect-compact__btn--danger" disabled={busy} onClick={() => void stopTunnel()}>
                <Power size={13} />
                {t("phone.tunnelStop")}
              </button>
            ) : (
              <button type="button" className="connect-compact__btn connect-compact__btn--primary" disabled={busy} onClick={() => void startTunnel()}>
                <Cloud size={13} />
                {t("phone.tunnelStart")}
              </button>
            )}
            {tunnelPhaseHint ? <p className="connect-compact__lan-hint">{tunnelPhaseHint}</p> : null}
          </div>
        </section>

        <section className="connect-compact__status">
          <div className="connect-compact__status-top">
            <div className="connect-compact__panel-head">
              <strong>{t("phone.statusTitle")}</strong>
              <p>{t("phone.statusHint")}</p>
            </div>

            <label className="connect-compact__switch-row">
              <span className="connect-compact__switch-copy">
                <strong>{t("phone.enabled")}</strong>
                <small>{t("phone.agentEnableHint")}</small>
              </span>
              <input
                type="checkbox"
                className="connect-compact__switch"
                checked={config.enabled}
                disabled={busy}
                onChange={(e) => void toggleEnabled(e.target.checked)}
              />
            </label>

            <label className="connect-compact__switch-row">
              <span className="connect-compact__switch-copy">
                <strong>{t("phone.allowLan")}</strong>
                <small>{t("phone.allowLanHint")}</small>
              </span>
              <input
                type="checkbox"
                className="connect-compact__switch"
                checked={Boolean(config.allowLAN)}
                disabled={busy}
                onChange={(e) => void toggleAllowLAN(e.target.checked)}
              />
            </label>
            <label className="connect-compact__switch-row">
              <span className="connect-compact__switch-copy">
                <strong>{t("phone.tunnelIdleShutdown")}</strong>
                <small>{t("phone.tunnelIdleShutdownHint", { min: tunnel.tunnelIdleTimeoutMin ?? 10 })}</small>
              </span>
              <input
                type="checkbox"
                className="connect-compact__switch"
                checked={tunnel.tunnelIdleAutoShutdown ?? !config.tunnelDisableIdleShutdown}
                disabled={busy}
                onChange={(e) => void toggleTunnelIdleShutdown(e.target.checked)}
              />
            </label>
          </div>

          <div className="connect-compact__status-main">
            <div className="connect-compact__block">
              <div className="connect-compact__kv-grid">
                <div className="connect-compact__kv">
                  <span className="connect-compact__kv-label">{t("phone.connectStatus")}</span>
                  <span className="connect-compact__kv-value">
                    <ModeIcon size={13} /> {mode.label}
                  </span>
                </div>
                <div className="connect-compact__kv">
                  <span className="connect-compact__kv-label">{t("phone.lanAddress")}</span>
                  <span className="connect-compact__kv-value">{lanAddress}</span>
                </div>
                {pairing?.relayUrl ? (
                  <div className="connect-compact__kv connect-compact__kv--full">
                    <span className="connect-compact__kv-label">{t("phone.relayBaseURL")}</span>
                    <span className={`connect-compact__kv-value${pairing.relayConnected ? " connect-compact__kv-value--ok" : ""}`}>
                      {pairing.relayConnected ? t("phone.relayOnline") : t("phone.relayOffline")}
                    </span>
                  </div>
                ) : null}
              </div>
            </div>

            <div className="connect-compact__block connect-compact__block--session">
              <strong className="connect-compact__block-title">{t("phone.desktopSessionTitle")}</strong>
              <p className="connect-compact__block-hint">{t("phone.desktopSessionHint")}</p>
              <div className="connect-compact__kv-grid">
                <div className="connect-compact__kv">
                  <span className="connect-compact__kv-label">{t("phone.desktopTab")}</span>
                  <span className="connect-compact__kv-value">{displayTab}</span>
                </div>
                <div className="connect-compact__kv">
                  <span className="connect-compact__kv-label">{t("phone.desktopModel")}</span>
                  <span className="connect-compact__kv-value">{currentModel || "—"}</span>
                </div>
                <div className="connect-compact__kv connect-compact__kv--full">
                  <span className="connect-compact__kv-label">{t("phone.desktopWorkspace")}</span>
                  <span className="connect-compact__kv-value" title={workspaceRoot}>
                    {displayWorkspace}
                  </span>
                </div>
              </div>
            </div>
          </div>

          <footer className="connect-compact__status-foot">
            <p className="connect-compact__footnote">{t("phone.remoteNote")}</p>
          </footer>
        </section>

        <section className="connect-compact__devices">
          <div className="connect-compact__panel-head">
            <strong>{t("phone.devicesTitle")}</strong>
            <p>{t("phone.devicesHint")}</p>
          </div>
          <div className="connect-compact__devices-box">
            {sessions.length ? (
              <ul className="connect-compact__devices-list">
                {sessions.slice(0, 6).map((item) => (
                  <li key={item.id}>
                    <span>{item.id.slice(0, 8)}…</span>
                    <time>{new Date(item.lastSeen).toLocaleTimeString()}</time>
                    <button
                      type="button"
                      className="connect-compact__revoke"
                      disabled={busy}
                      onClick={() => void revokeDevice(item.id)}
                    >
                      {t("phone.revokeDevice")}
                    </button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="connect-compact__devices-empty">{t("phone.noDevices")}</p>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
