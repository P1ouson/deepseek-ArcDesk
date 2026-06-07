import { useCallback, useEffect, useState } from "react";
import { Loader2, Shield } from "lucide-react";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { ConfigureProjectSandboxInput, ProjectSandboxStatus } from "../lib/types";

export interface SandboxSetupOverlayProps {
  reason: "yolo" | "manual";
  onComplete: () => void;
  onCancel: () => void;
}

export function SandboxSetupOverlay({ reason, onComplete, onCancel }: SandboxSetupOverlayProps) {
  const t = useT();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [status, setStatus] = useState<ProjectSandboxStatus | null>(null);
  const [bash, setBash] = useState("enforce");
  const [network, setNetwork] = useState(true);
  const [previewHosts, setPreviewHosts] = useState("localhost,127.0.0.1");
  const [previewPorts, setPreviewPorts] = useState("5173,3000,8080");

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const next = await app.ProjectSandboxStatus();
        if (cancelled) return;
        setStatus(next);
        setBash(next.bash || "enforce");
        setNetwork(next.network);
        setPreviewHosts((next.previewHosts ?? []).join(", "));
        setPreviewPorts((next.previewPorts ?? []).join(", "));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const submit = useCallback(async () => {
    setSaving(true);
    setError("");
    const ports = previewPorts
      .split(/[,\s]+/)
      .map((v) => Number.parseInt(v, 10))
      .filter((n) => Number.isFinite(n));
    const input: ConfigureProjectSandboxInput = {
      bash,
      network,
      allowWrite: status?.allowWrite ?? [],
      previewHosts: previewHosts.split(/[,\s]+/).map((v) => v.trim()).filter(Boolean),
      previewPorts: ports,
    };
    try {
      await app.ConfigureProjectSandbox(input);
      onComplete();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("sandboxSetup.errorSave"));
    } finally {
      setSaving(false);
    }
  }, [bash, network, onComplete, previewHosts, previewPorts, status?.allowWrite, t]);

  return (
    <div className="sandbox-setup">
      <div className="sandbox-setup__card">
        <div className="sandbox-setup__head">
          <Shield size={18} />
          <div>
            <strong>{t("sandboxSetup.title")}</strong>
            <p>{reason === "yolo" ? t("sandboxSetup.subtitleYolo") : t("sandboxSetup.subtitle")}</p>
          </div>
        </div>

        {loading ? (
          <div className="sandbox-setup__loading">
            <Loader2 size={16} />
            <span>{t("sandboxSetup.loading")}</span>
          </div>
        ) : (
          <>
            <label className="sandbox-setup__field">
              <span>{t("sandboxSetup.workspace")}</span>
              <input value={status?.workspaceRoot ?? ""} readOnly />
            </label>

            <label className="sandbox-setup__field">
              <span>{t("sandboxSetup.bash")}</span>
              <select value={bash} onChange={(e) => setBash(e.target.value)}>
                <option value="enforce">{t("settings.bashEnforce")}</option>
                <option value="off">{t("settings.bashOff")}</option>
              </select>
            </label>

            <label className="sandbox-setup__check">
              <input type="checkbox" checked={network} onChange={(e) => setNetwork(e.target.checked)} />
              <span>{t("settings.allowNetwork")}</span>
            </label>

            <label className="sandbox-setup__field">
              <span>{t("sandboxSetup.previewHosts")}</span>
              <input value={previewHosts} onChange={(e) => setPreviewHosts(e.target.value)} spellCheck={false} />
              <small>{t("sandboxSetup.previewHostsHint")}</small>
            </label>

            <label className="sandbox-setup__field">
              <span>{t("sandboxSetup.previewPorts")}</span>
              <input value={previewPorts} onChange={(e) => setPreviewPorts(e.target.value)} spellCheck={false} />
              <small>{t("sandboxSetup.previewPortsHint")}</small>
            </label>

            <p className="sandbox-setup__note">{t("sandboxSetup.webNote")}</p>

            {error && (
              <div className="sandbox-setup__error" role="alert">
                {error}
              </div>
            )}

            <div className="sandbox-setup__actions">
              <button type="button" className="sandbox-setup__secondary" onClick={onCancel} disabled={saving}>
                {t("common.cancel")}
              </button>
              <button type="button" className="sandbox-setup__primary" onClick={() => void submit()} disabled={saving}>
                {saving
                  ? t("sandboxSetup.saving")
                  : reason === "yolo"
                    ? t("sandboxSetup.save")
                    : t("common.save")}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
