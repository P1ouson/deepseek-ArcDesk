import { useT } from "../lib/i18n";
import { CollapsibleCommaField, CollapsiblePreviewPortsField } from "./CollapsibleCommaField";

export interface ProjectPreviewSettingsProps {
  previewStrict: boolean;
  onPreviewStrictChange: (value: boolean) => void;
  previewHosts: string[];
  onPreviewHostsChange: (hosts: string[]) => void;
  previewPorts: number[];
  onPreviewPortsChange: (ports: number[]) => void;
  busy?: boolean;
}

export function ProjectPreviewSettings({
  previewStrict,
  onPreviewStrictChange,
  previewHosts,
  onPreviewHostsChange,
  previewPorts,
  onPreviewPortsChange,
  busy = false,
}: ProjectPreviewSettingsProps) {
  const t = useT();

  return (
    <div className="project-preview-settings">
      <label className="set-check project-preview-settings__strict">
        <input
          type="checkbox"
          checked={previewStrict}
          disabled={busy}
          onChange={(e) => onPreviewStrictChange(e.target.checked)}
        />
        {t("settings.permissions.previewStrict")}
      </label>

      <CollapsibleCommaField
        title={t("settings.permissions.previewHosts")}
        values={previewHosts}
        placeholder={t("settings.permissions.previewHostPlaceholder")}
        hint={t("settings.permissions.previewHostsHint")}
        busy={busy}
        onCommit={onPreviewHostsChange}
      />

      <CollapsiblePreviewPortsField
        title={t("settings.permissions.previewPorts")}
        ports={previewPorts}
        placeholder={t("settings.permissions.previewPortPlaceholder")}
        hint={t("settings.permissions.previewPortsHint")}
        busy={busy}
        onCommit={onPreviewPortsChange}
      />
    </div>
  );
}
