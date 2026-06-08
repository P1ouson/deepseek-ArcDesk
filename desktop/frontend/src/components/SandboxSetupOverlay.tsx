import { useCallback, useEffect, useState } from "react";
import { Loader2, Shield } from "lucide-react";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { ConfigureProjectSandboxInput, ProjectSandboxStatus } from "../lib/types";
import { ProjectPreviewSettings } from "./ProjectPreviewSettings";
import { StudioSelect } from "./StudioSelect";

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
  const [previewHosts, setPreviewHosts] = useState<string[]>([]);
  const [previewPorts, setPreviewPorts] = useState<number[]>([]);
  const [previewStrict, setPreviewStrict] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const next = await app.ProjectSandboxStatus();
        if (cancelled) return;
        setStatus(next);
        setBash(next.bash || "enforce");
        setNetwork(next.network);
        setPreviewHosts(next.previewHosts ?? []);
        setPreviewPorts(next.previewPorts ?? []);
        setPreviewStrict(next.previewStrict ?? false);
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
    const input: ConfigureProjectSandboxInput = {
      bash,
      network,
      allowWrite: status?.allowWrite ?? [],
      previewHosts,
      previewPorts,
      previewStrict,
    };
    try {
      await app.ConfigureProjectSandbox(input);
      onComplete();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("sandboxSetup.errorSave"));
    } finally {
      setSaving(false);
    }
  }, [bash, network, onComplete, previewHosts, previewPorts, previewStrict, status?.allowWrite, t]);

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
              <StudioSelect
                value={bash}
                onChange={setBash}
                options={[
                  { value: "enforce", label: t("settings.bashEnforce") },
                  { value: "off", label: t("settings.bashOff") },
                ]}
              />
            </label>

            <label className="sandbox-setup__check">
              <input type="checkbox" checked={network} onChange={(e) => setNetwork(e.target.checked)} />
              <span>{t("settings.allowNetwork")}</span>
            </label>

            <div className="sandbox-setup__section">
              <span className="sandbox-setup__section-title">{t("settings.permissions.previewTitle")}</span>
              <ProjectPreviewSettings
                busy={saving}
                previewStrict={previewStrict}
                onPreviewStrictChange={setPreviewStrict}
                previewHosts={previewHosts}
                onPreviewHostsChange={setPreviewHosts}
                previewPorts={previewPorts}
                onPreviewPortsChange={setPreviewPorts}
              />
            </div>

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
