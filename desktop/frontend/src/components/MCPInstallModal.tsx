import { useCallback, useEffect, useMemo, useState } from "react";
import { AlertTriangle, CheckCircle2, KeyRound, Loader2, Plug, Server } from "lucide-react";
import { app } from "../lib/bridge";
import { toErrorMessage } from "../lib/errors";
import { useT, type DictKey } from "../lib/i18n";
import type { MCPCatalogEntry, MCPCatalogEnvField, MCPServerInput, MCPPrerequisiteView } from "../lib/types";
import { StudioCenterModal } from "./StudioCenterModal";

function inferredRequires(entry: MCPCatalogEntry): string[] {
  if (entry.requires?.length) return entry.requires;
  const transport = (entry.transport ?? "stdio").toLowerCase();
  const command = (entry.command ?? "").toLowerCase();
  if ((transport === "" || transport === "stdio") && command === "npx") return ["node"];
  return [];
}

function catalogToInput(entry: MCPCatalogEntry, env: Record<string, string>): MCPServerInput {
  const cleaned: Record<string, string> = {};
  for (const [key, value] of Object.entries(env)) {
    const trimmed = value.trim();
    if (trimmed) cleaned[key] = trimmed;
  }
  return {
    name: entry.id,
    transport: entry.transport,
    command: entry.command ?? "",
    args: entry.args ?? [],
    url: entry.url ?? "",
    tier: entry.tier ?? "lazy",
    env: Object.keys(cleaned).length ? cleaned : undefined,
    confirmed: true,
  };
}

function envFieldLabel(t: ReturnType<typeof useT>, entryId: string, field: MCPCatalogEnvField): string {
  const specificKey = `extensions.mcpSetup.field.${entryId}.${field.key}.label` as DictKey;
  const specific = t(specificKey);
  if (specific !== specificKey) return specific;
  const genericKey = `extensions.mcpSetup.field.${field.key}.label` as DictKey;
  const genericLabel = t(genericKey);
  if (genericLabel !== genericKey) return genericLabel;
  return field.key;
}

function envFieldHint(t: ReturnType<typeof useT>, entryId: string, field: MCPCatalogEnvField): string | null {
  const specificKey = `extensions.mcpSetup.field.${entryId}.${field.key}.hint` as DictKey;
  const specific = t(specificKey);
  if (specific !== specificKey) return specific;
  const genericKey = `extensions.mcpSetup.field.${field.key}.hint` as DictKey;
  const genericHint = t(genericKey);
  if (genericHint !== genericKey) return genericHint;
  return null;
}

export function MCPInstallModal({
  entry,
  mode,
  onClose,
  onDone,
}: {
  entry: MCPCatalogEntry;
  mode: "install" | "configure";
  onClose: () => void;
  onDone: () => void;
}) {
  const t = useT();
  const requires = useMemo(() => inferredRequires(entry), [entry]);
  const envFields = entry.envFields ?? [];
  const [prereqs, setPrereqs] = useState<MCPPrerequisiteView[]>([]);
  const [prereqLoading, setPrereqLoading] = useState(true);
  const [env, setEnv] = useState<Record<string, string>>(() =>
    Object.fromEntries(envFields.map((field) => [field.key, ""])),
  );
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const refreshPrereqs = useCallback(async () => {
    setPrereqLoading(true);
    try {
      const view = await app.CheckMCPPrerequisites(requires);
      setPrereqs(view.items ?? []);
    } catch (e) {
      setPrereqs([]);
      setErr(toErrorMessage(e));
    } finally {
      setPrereqLoading(false);
    }
  }, [requires]);

  useEffect(() => {
    void refreshPrereqs();
  }, [refreshPrereqs]);

  const nodePrereq = prereqs.find((item) => item.id === "node");
  const nodeRequired = requires.includes("node");
  const nodeMissing = nodeRequired && nodePrereq && !nodePrereq.ok;
  const missingRequiredEnv = envFields.some((field) => field.required && !env[field.key]?.trim());
  const canSubmit = !busy && !nodeMissing && !missingRequiredEnv;

  const submit = async () => {
    setBusy(true);
    setErr(null);
    try {
      const input = catalogToInput(entry, env);
      if (mode === "configure") {
        await app.UpdateMCPServer(entry.id, input);
      } else {
        await app.AddMCPServer(input);
      }
      onDone();
      onClose();
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <StudioCenterModal
      wide
      className="mcp-setup-modal"
      title={
        mode === "configure"
          ? t("extensions.mcpSetup.configureTitle", { name: entry.name })
          : t("extensions.mcpSetup.title", { name: entry.name })
      }
      titleId="mcp-setup-title"
      onClose={onClose}
    >
      <div className="mcp-setup">
        <header className="mcp-setup__hero">
          <div className="mcp-setup__hero-icon" aria-hidden="true">
            <Plug size={22} />
          </div>
          <div className="mcp-setup__hero-copy">
            <div className="mcp-setup__hero-head">
              <strong>{entry.name}</strong>
              <span className="mcp-setup__chip">{entry.category}</span>
              {entry.official ? <span className="mcp-setup__chip mcp-setup__chip--accent">{t("plugins.official")}</span> : null}
            </div>
            <p className="mcp-setup__hero-desc">{entry.description}</p>
            <p className="mcp-setup__hero-sub">{t("extensions.mcpSetup.subtitle")}</p>
          </div>
        </header>

        {nodeRequired ? (
          <section className="mcp-setup__card">
            <div className="mcp-setup__card-head">
              <Server size={16} />
              <h4>{t("extensions.mcpSetup.prereqHeading")}</h4>
            </div>
            {prereqLoading ? (
              <div className="mcp-setup__checking">
                <Loader2 size={16} className="dock-panel__spin" />
                {t("extensions.mcpSetup.checking")}
              </div>
            ) : nodePrereq?.ok ? (
              <div className="mcp-setup__prereq mcp-setup__prereq--ok">
                <CheckCircle2 size={16} />
                <span>{t("extensions.mcpSetup.prereqNodeOk", { version: nodePrereq.detail || "OK" })}</span>
              </div>
            ) : (
              <div className="mcp-setup__prereq mcp-setup__prereq--warn">
                <AlertTriangle size={16} />
                <div>
                  <strong>{t("extensions.mcpSetup.prereqNodeMissing")}</strong>
                  <p>{t("extensions.mcpSetup.prereqNodeHint")}</p>
                </div>
              </div>
            )}
          </section>
        ) : null}

        {entry.setupNotes?.length ? (
          <section className="mcp-setup__card">
            <div className="mcp-setup__card-head">
              <AlertTriangle size={16} />
              <h4>{t("extensions.mcpSetup.notesHeading")}</h4>
            </div>
            <ul className="mcp-setup__notes">
              {entry.setupNotes.map((note) => {
                const noteKey = `extensions.mcpSetup.note.${note}` as DictKey;
                return <li key={note}>{t(noteKey)}</li>;
              })}
            </ul>
          </section>
        ) : null}

        {envFields.length ? (
          <section className="mcp-setup__card mcp-setup__card--credentials">
            <div className="mcp-setup__card-head">
              <KeyRound size={16} />
              <h4>{t("extensions.mcpSetup.credentialsHeading")}</h4>
            </div>
            {mode === "configure" ? <p className="mcp-setup__hint">{t("extensions.mcpSetup.configureHint")}</p> : null}
            <div className="mcp-setup__fields">
              {envFields.map((field) => {
                const hint = envFieldHint(t, entry.id, field);
                const label = envFieldLabel(t, entry.id, field);
                return (
                  <label key={field.key} className="mcp-setup__field">
                    <span className="mcp-setup__field-label">
                      {label}
                      {field.required ? <span className="mcp-setup__required">*</span> : null}
                    </span>
                    <input
                      type={field.secret ? "password" : "text"}
                      value={env[field.key] ?? ""}
                      autoComplete="off"
                      spellCheck={false}
                      placeholder={field.secret ? t("extensions.mcpSetup.secretPlaceholder") : field.key}
                      onChange={(e) => setEnv((prev) => ({ ...prev, [field.key]: e.target.value }))}
                    />
                    {hint ? <span className="mcp-setup__field-hint">{hint}</span> : null}
                  </label>
                );
              })}
            </div>
          </section>
        ) : !nodeRequired && !entry.setupNotes?.length ? (
          <p className="mcp-setup__hint mcp-setup__hint--center">{t("extensions.mcpSetup.noCredentials")}</p>
        ) : null}

        <p className="mcp-setup__footer-hint">{t("extensions.mcpSetup.footerHint")}</p>

        {err ? <div className="mcp-setup__banner mcp-setup__banner--error">{err}</div> : null}

        <div className="mcp-setup__actions">
          <button type="button" className="mcp-setup__btn" disabled={busy} onClick={onClose}>
            {t("common.cancel")}
          </button>
          <button type="button" className="mcp-setup__btn mcp-setup__btn--primary" disabled={!canSubmit} onClick={() => void submit()}>
            {busy ? <Loader2 size={14} className="dock-panel__spin" /> : null}
            {mode === "configure" ? t("common.save") : t("plugins.install")}
          </button>
        </div>
      </div>
    </StudioCenterModal>
  );
}
