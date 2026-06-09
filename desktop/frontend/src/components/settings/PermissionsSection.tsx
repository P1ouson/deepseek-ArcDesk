import { useEffect, useState } from "react";
import { app } from "../../lib/bridge";
import { logBridgeError } from "../../lib/logBridgeError";
import { useT } from "../../lib/i18n";
import { ProjectPreviewSettings } from "../ProjectPreviewSettings";
import { RuleList } from "../RuleList";
import { StudioSelect } from "../StudioSelect";
import { SettingsBlock, SettingsSaveChip, type SettingsSectionProps } from "../settingsPrimitives";

const PERMISSION_RULE_LISTS = ["deny", "ask", "allow"] as const;

export function PermissionsSection({ s, busy, apply }: SettingsSectionProps) {
  const t = useT();
  const perms = s.permissions;
  const sb = s.sandbox;
  const [root, setRoot] = useState(sb.workspaceRoot);
  const [previewHosts, setPreviewHosts] = useState<string[]>([]);
  const [previewPorts, setPreviewPorts] = useState<number[]>([]);
  const [previewStrict, setPreviewStrict] = useState(false);
  const [savedPreview, setSavedPreview] = useState({ hosts: [] as string[], ports: [] as number[], strict: false });

  useEffect(() => {
    setRoot(sb.workspaceRoot);
  }, [sb.workspaceRoot]);

  useEffect(() => {
    let cancelled = false;
    void app
      .ProjectSandboxStatus()
      .then((status) => {
        if (cancelled) return;
        const hosts = status.previewHosts ?? [];
        const ports = status.previewPorts ?? [];
        const strict = status.previewStrict ?? false;
        setPreviewHosts(hosts);
        setPreviewPorts(ports);
        setPreviewStrict(strict);
        setSavedPreview({ hosts, ports, strict });
      })
      .catch((err) => logBridgeError("PreviewSettings", err));
    return () => {
      cancelled = true;
    };
  }, [s.configPath]);

  const setSandbox = (next: Partial<typeof sb>) =>
    apply(() =>
      app.SetSandbox(
        next.bash ?? sb.bash,
        next.network ?? sb.network,
        next.workspaceRoot ?? sb.workspaceRoot,
        next.allowWrite ?? sb.allowWrite,
      ),
    );

  const previewDirty =
    previewStrict !== savedPreview.strict ||
    previewHosts.join("\0") !== savedPreview.hosts.join("\0") ||
    previewPorts.join(",") !== savedPreview.ports.join(",");

  const savePreview = () => {
    void apply(async () => {
      await app.SaveProjectPreviewSettings({
        previewHosts,
        previewPorts,
        previewStrict,
      });
      setSavedPreview({ hosts: [...previewHosts], ports: [...previewPorts], strict: previewStrict });
    });
  };

  return (
    <>
      <SettingsBlock title={t("settings.permissions.writerModeTitle")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.permissions.writerFallback")}</label>
            <StudioSelect
              className="set-grow"
              value={perms.mode}
              disabled={busy}
              onChange={(value) => void apply(() => app.SetPermissionMode(value))}
              options={[
                { value: "ask", label: t("settings.modeAsk") },
                { value: "allow", label: t("settings.modeAllow") },
                { value: "deny", label: t("settings.modeDeny") },
              ]}
            />
          </div>
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.permissions.rulesTitle")}>
        <div className="settings-block__form">
          {PERMISSION_RULE_LISTS.map((list) => (
            <RuleList
              key={list}
              collapsible
              list={list}
              rules={perms[list]}
              busy={busy}
              title={t(`settings.permissions.list.${list}`)}
              tone={list}
              placeholder={t("settings.permissions.addRulePlaceholder")}
              onAdd={(rule) => apply(() => app.AddPermissionRule(list, rule))}
              onRemove={(rule) => apply(() => app.RemovePermissionRule(list, rule))}
            />
          ))}
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.sandboxTitle")}>
        <div className="settings-block__form">
          <div className="set-row">
            <label className="set-label">{t("settings.bashSandbox")}</label>
            <StudioSelect
              className="set-grow"
              value={sb.bash}
              disabled={busy}
              onChange={(value) => void setSandbox({ bash: value })}
              options={[
                { value: "enforce", label: t("settings.bashEnforce") },
                { value: "off", label: t("settings.bashOff") },
              ]}
            />
          </div>
          <label className="set-check">
            <input type="checkbox" checked={sb.network} disabled={busy} onChange={(e) => void setSandbox({ network: e.target.checked })} />
            {t("settings.allowNetwork")}
          </label>
          <div className="set-row">
            <label className="set-label">{t("settings.workspaceRoot")}</label>
            <input
              className="mem-input set-grow"
              placeholder={t("settings.workspaceDefault")}
              value={root}
              disabled={busy}
              onChange={(e) => setRoot(e.target.value)}
              onBlur={() => root !== sb.workspaceRoot && void setSandbox({ workspaceRoot: root })}
            />
          </div>
          <RuleList
            collapsible
            list="allow_write"
            rules={sb.allowWrite}
            busy={busy}
            title={t("settings.permissions.allowWrite")}
            placeholder={t("settings.permissions.allowWritePlaceholder")}
            onAdd={(dir) => setSandbox({ allowWrite: [...sb.allowWrite, dir] })}
            onRemove={(dir) => setSandbox({ allowWrite: sb.allowWrite.filter((x) => x !== dir) })}
          />
        </div>
      </SettingsBlock>

      <SettingsBlock title={t("settings.permissions.previewTitle")}>
        <div className="settings-block__form">
          <ProjectPreviewSettings
            busy={busy}
            previewStrict={previewStrict}
            onPreviewStrictChange={setPreviewStrict}
            previewHosts={previewHosts}
            onPreviewHostsChange={setPreviewHosts}
            previewPorts={previewPorts}
            onPreviewPortsChange={setPreviewPorts}
          />
          <div className="settings-permissions-preview-save">
            <SettingsSaveChip disabled={busy || !previewDirty} ready={previewDirty} onClick={savePreview}>
              {t("settings.permissions.savePreview")}
            </SettingsSaveChip>
          </div>
        </div>
      </SettingsBlock>
    </>
  );
}
