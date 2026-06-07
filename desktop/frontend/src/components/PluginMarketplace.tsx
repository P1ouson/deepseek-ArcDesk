import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, Download, Puzzle, RefreshCw, Search, ShieldCheck } from "lucide-react";
import { app } from "../lib/bridge";
import { useT } from "../lib/i18n";
import type { CapabilitiesView, MCPCatalogEntry, MCPServerInput, ServerView, SkillView } from "../lib/types";

type PluginTab = "marketplace" | "installed" | "skills";

function serverStatusLabel(status: ServerView["status"], t: ReturnType<typeof useT>): string {
  switch (status) {
    case "connected":
      return t("plugins.status.connected");
    case "failed":
      return t("plugins.status.failed");
    case "disabled":
      return t("plugins.status.disabled");
    case "initializing":
      return t("plugins.status.starting");
    case "deferred":
      return t("plugins.status.deferred");
    default:
      return status;
  }
}

function catalogToInput(entry: MCPCatalogEntry): MCPServerInput {
  return {
    name: entry.id,
    transport: entry.transport,
    command: entry.command ?? "",
    args: entry.args ?? [],
    url: entry.url ?? "",
    tier: entry.tier ?? "lazy",
  };
}

export function PluginMarketplace() {
  const t = useT();
  const [tab, setTab] = useState<PluginTab>("marketplace");
  const [catalog, setCatalog] = useState<MCPCatalogEntry[]>([]);
  const [caps, setCaps] = useState<CapabilitiesView | null>(null);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const [catalogItems, capabilities] = await Promise.all([
      app.ListMCPCatalog().catch(() => [] as MCPCatalogEntry[]),
      app.Capabilities().catch(() => ({ servers: [], skills: [], skillRoots: [] })),
    ]);
    setCatalog(catalogItems);
    setCaps(capabilities);
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const installedNames = useMemo(() => new Set((caps?.servers ?? []).map((s) => s.name)), [caps?.servers]);

  const filteredCatalog = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return catalog;
    return catalog.filter((entry) =>
      [entry.name, entry.category, entry.description, entry.transport].join(" ").toLowerCase().includes(q),
    );
  }, [catalog, query]);

  const filteredServers = useMemo(() => {
    const servers = caps?.servers ?? [];
    const q = query.trim().toLowerCase();
    if (!q) return servers;
    return servers.filter((s) => [s.name, s.transport, s.status, s.error ?? ""].join(" ").toLowerCase().includes(q));
  }, [caps?.servers, query]);

  const filteredSkills = useMemo(() => {
    const skills = caps?.skills ?? [];
    const q = query.trim().toLowerCase();
    if (!q) return skills;
    return skills.filter((s) => [s.name, s.description, s.scope, s.runAs].join(" ").toLowerCase().includes(q));
  }, [caps?.skills, query]);

  const run = async (fn: () => Promise<void>, success?: string) => {
    setBusy(true);
    setErr(null);
    setNotice(null);
    try {
      await fn();
      await reload();
      if (success) setNotice(success);
    } catch (e) {
      setErr(String((e as Error)?.message ?? e));
      await reload();
    } finally {
      setBusy(false);
    }
  };

  const install = (entry: MCPCatalogEntry) =>
    run(async () => {
      await app.AddMCPServer(catalogToInput(entry));
    }, t("plugins.installedOk", { name: entry.name }));

  const toggleServer = (name: string, enabled: boolean) =>
    run(async () => {
      await app.SetMCPServerEnabled(name, enabled);
    });

  const retryServer = (name: string) =>
    run(async () => {
      await app.RetryMCPServer(name);
    });

  const removeServer = (name: string) =>
    run(async () => {
      await app.RemoveMCPServer(name);
    });

  const toggleSkill = (skill: SkillView) =>
    run(async () => {
      await app.SetSkillEnabled(skill.name, !skill.enabled);
    });

  return (
    <div className="plugin-marketplace">
      <header className="plugin-marketplace__head">
        <div>
          <div className="plugin-marketplace__title">
            <Puzzle size={18} />
            {t("plugins.title")}
          </div>
          <div className="plugin-marketplace__sub">{t("plugins.subtitle")}</div>
        </div>
        <button type="button" className="plugin-marketplace__refresh" disabled={busy} onClick={() => void reload()}>
          <RefreshCw size={15} />
          {t("plugins.refresh")}
        </button>
      </header>

      <div className="plugin-marketplace__toolbar">
        <div className="plugin-marketplace__tabs">
          {(
            [
              ["marketplace", t("plugins.tab.marketplace")],
              ["installed", t("plugins.tab.installed")],
              ["skills", t("plugins.tab.skills")],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              className={`plugin-marketplace__tab${tab === id ? " plugin-marketplace__tab--active" : ""}`}
              onClick={() => setTab(id)}
            >
              {label}
            </button>
          ))}
        </div>
        <label className="plugin-marketplace__search">
          <Search size={15} />
          <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t("plugins.searchPlaceholder")} />
        </label>
      </div>

      {err ? <div className="plugin-marketplace__banner plugin-marketplace__banner--error">{err}</div> : null}
      {notice ? <div className="plugin-marketplace__banner plugin-marketplace__banner--ok">{notice}</div> : null}

      {tab === "marketplace" ? (
        <div className="plugin-marketplace__grid">
          {filteredCatalog.map((entry) => {
            const installed = installedNames.has(entry.id) || installedNames.has(entry.name.toLowerCase());
            return (
              <article key={entry.id} className="plugin-marketplace__card">
                <div className="plugin-marketplace__card-head">
                  <strong>{entry.name}</strong>
                  <span className="plugin-marketplace__chip">{entry.category}</span>
                </div>
                <p>{entry.description}</p>
                <div className="plugin-marketplace__meta">
                  <span>{entry.transport}</span>
                  {entry.official ? (
                    <span className="plugin-marketplace__official">
                      <ShieldCheck size={13} /> {t("plugins.official")}
                    </span>
                  ) : null}
                </div>
                <button
                  type="button"
                  className={`plugin-marketplace__action${installed ? " plugin-marketplace__action--installed" : ""}`}
                  disabled={busy || installed}
                  onClick={() => void install(entry)}
                >
                  {installed ? (
                    <>
                      <Check size={14} /> {t("plugins.installed")}
                    </>
                  ) : (
                    <>
                      <Download size={14} /> {t("plugins.install")}
                    </>
                  )}
                </button>
              </article>
            );
          })}
          {!filteredCatalog.length ? <div className="plugin-marketplace__empty">{t("plugins.emptyMarketplace")}</div> : null}
        </div>
      ) : null}

      {tab === "installed" ? (
        <div className="plugin-marketplace__list">
          {filteredServers.map((server) => (
            <article key={server.name} className="plugin-marketplace__row">
              <div className="plugin-marketplace__row-main">
                <strong>{server.name}</strong>
                <span>{server.transport}</span>
                <span className={`plugin-marketplace__status plugin-marketplace__status--${server.status}`}>
                  {serverStatusLabel(server.status, t)}
                </span>
                {server.tools ? <span>{t("plugins.tools", { n: server.tools })}</span> : null}
                {server.error ? <span className="plugin-marketplace__error">{server.error}</span> : null}
              </div>
              <div className="plugin-marketplace__row-actions">
                {server.status === "failed" ? (
                  <button type="button" disabled={busy} onClick={() => void retryServer(server.name)}>
                    {t("plugins.retry")}
                  </button>
                ) : null}
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void toggleServer(server.name, server.status === "disabled")}
                >
                  {server.status === "disabled" ? t("plugins.enable") : t("plugins.disable")}
                </button>
                {!server.builtIn ? (
                  <button type="button" className="plugin-marketplace__danger" disabled={busy} onClick={() => void removeServer(server.name)}>
                    {t("plugins.remove")}
                  </button>
                ) : null}
              </div>
            </article>
          ))}
          {!filteredServers.length ? <div className="plugin-marketplace__empty">{t("plugins.emptyInstalled")}</div> : null}
        </div>
      ) : null}

      {tab === "skills" ? (
        <div className="plugin-marketplace__list">
          {filteredSkills.map((skill) => (
            <article key={skill.name} className="plugin-marketplace__row">
              <div className="plugin-marketplace__row-main">
                <strong>/{skill.name}</strong>
                <span>{skill.scope}</span>
                <span>{skill.runAs}</span>
                <span>{skill.description}</span>
              </div>
              <div className="plugin-marketplace__row-actions">
                <button type="button" disabled={busy} onClick={() => void toggleSkill(skill)}>
                  {skill.enabled ? t("plugins.disable") : t("plugins.enable")}
                </button>
              </div>
            </article>
          ))}
          {!filteredSkills.length ? <div className="plugin-marketplace__empty">{t("plugins.emptySkills")}</div> : null}
        </div>
      ) : null}
    </div>
  );
}
