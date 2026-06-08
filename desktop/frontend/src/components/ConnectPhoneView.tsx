import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  Cloud,
  Copy,
  Globe,
  Loader2,
  Power,
  QrCode,
  RefreshCw,
  Smartphone,
  Wifi,
} from "lucide-react";
import { asArray } from "../lib/array";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { MobileConnectConfig, MobilePairingInfo, MobileTunnelStatus, ModelInfo } from "../lib/types";

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
    default:
      return { label: t("phone.modeNone"), icon: Globe, ok: false };
  }
}

function basename(path: string): string {
  const normalized = path.replace(/\\/g, "/").replace(/\/+$/, "");
  const parts = normalized.split("/").filter(Boolean);
  return parts[parts.length - 1] || path || "—";
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
    model: "",
    persona: "",
    workspaceRoot,
    relayBaseURL: "",
  }));
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [copiedField, setCopiedField] = useState<"pair" | "tunnel" | null>(null);
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
    const timer = window.setInterval(() => void reload(), 5_000);
    return () => window.clearInterval(timer);
  }, [reload]);

  useEffect(() => {
    if (!tunnel.running || tunnel.url) return;
    const timer = window.setInterval(() => void reload(), 2_000);
    return () => window.clearInterval(timer);
  }, [tunnel.running, tunnel.url, reload]);

  useEffect(() => {
    const off = window.runtime?.EventsOn?.("mobile:message", () => void reload());
    return () => off?.();
  }, [reload]);

  const mode = useMemo(() => modeMeta(pairing?.connectMode, t), [pairing?.connectMode, t]);
  const ModeIcon = mode.icon;
  const tunnelReady = Boolean(tunnel.url || pairing?.tunnelUrl);
  const tunnelUrl = tunnel.url || pairing?.tunnelUrl || "";
  const lanAddress = pairing?.lanIp ? `${pairing.lanIp}:${pairing.port}` : "—";
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

  const toggleEnabled = async (enabled: boolean) => {
    const prev = config.enabled;
    const nextConfig = {
      ...config,
      enabled,
      workspaceRoot,
      relayBaseURL: config.relayBaseURL?.trim() ?? "",
    };
    setConfig(nextConfig);
    setBusy(true);
    setErr(null);
    try {
      await app.SaveMobileConnectConfig(nextConfig);
      setNotice(t("phone.saved"));
      await reload();
    } catch (e) {
      setConfig((v) => ({ ...v, enabled: prev }));
      setErr(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const refreshQR = async () => {
    setBusy(true);
    setErr(null);
    try {
      setPairing(await app.RefreshMobilePairing());
      setNotice(t("phone.qrRefreshed"));
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
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
        setErr(status.err);
        return;
      }
      setNotice(t("phone.tunnelStarting"));
      await reload();
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
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
      setErr(String((e as Error)?.message ?? e));
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
          <span className="connect-compact__chip connect-compact__chip--ok">
            <Smartphone size={12} /> {t("phone.pairedDevices", { n: pairing?.pairedCount ?? sessions.length })}
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
              <div className="connect-compact__qr-empty">{t("phone.qrUnavailable")}</div>
            )}
          </div>
          {pairing?.pairUrl ? <code className="connect-compact__url">{pairing.pairUrl}</code> : null}
          <div className="connect-compact__tunnel">
            {tunnel.running ? (
              <button type="button" className="connect-compact__btn connect-compact__btn--danger" disabled={busy} onClick={() => void stopTunnel()}>
                {busy ? <Loader2 size={13} className="spin" /> : <Power size={13} />}
                {t("phone.tunnelStop")}
              </button>
            ) : (
              <button type="button" className="connect-compact__btn connect-compact__btn--primary" disabled={busy} onClick={() => void startTunnel()}>
                {busy ? <Loader2 size={13} className="spin" /> : <Cloud size={13} />}
                {t("phone.tunnelStart")}
              </button>
            )}
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
                {pairing?.pairUrl ? (
                  <div className="connect-compact__kv connect-compact__kv--full">
                    <span className="connect-compact__kv-label">{t("phone.pairAddress")}</span>
                    <div className="connect-compact__url-row">
                      <code>{pairing.pairUrl}</code>
                      <button
                        type="button"
                        className={`connect-compact__copy-btn${copiedField === "pair" ? " connect-compact__copy-btn--ok" : ""}`}
                        onClick={() => void copyText(pairing.pairUrl, "pair")}
                        title={copiedField === "pair" ? t("phone.copied") : t("common.copy")}
                        aria-label={copiedField === "pair" ? t("phone.copied") : t("common.copy")}
                      >
                        {copiedField === "pair" ? <Check size={12} /> : <Copy size={12} />}
                      </button>
                    </div>
                  </div>
                ) : null}
                {tunnelUrl ? (
                  <div className="connect-compact__kv connect-compact__kv--full">
                    <span className="connect-compact__kv-label">{t("phone.tunnelAddress")}</span>
                    <div className="connect-compact__url-row">
                      <code>{tunnelUrl}</code>
                      <button
                        type="button"
                        className={`connect-compact__copy-btn${copiedField === "tunnel" ? " connect-compact__copy-btn--ok" : ""}`}
                        onClick={() => void copyText(tunnelUrl, "tunnel")}
                        title={copiedField === "tunnel" ? t("phone.copied") : t("common.copy")}
                        aria-label={copiedField === "tunnel" ? t("phone.copied") : t("common.copy")}
                      >
                        {copiedField === "tunnel" ? <Check size={12} /> : <Copy size={12} />}
                      </button>
                    </div>
                  </div>
                ) : null}
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
          <ul>
            {sessions.length ? (
              sessions.slice(0, 4).map((item) => (
                <li key={item.id}>
                  <span>{item.id.slice(0, 8)}…</span>
                  <time>{new Date(item.lastSeen).toLocaleTimeString()}</time>
                </li>
              ))
            ) : (
              <li className="connect-compact__devices-empty">{t("phone.noDevices")}</li>
            )}
          </ul>
          <p>{t("phone.tunnelHint")}</p>
        </section>
      </div>
    </div>
  );
}
