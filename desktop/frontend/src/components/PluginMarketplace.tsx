import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Check,
  ChevronDown,
  ChevronRight,
  Download,
  ExternalLink,
  FolderPlus,
  Loader2,
  Plug,
  Plus,
  RefreshCw,
  Search,
  ShieldCheck,
  Sparkles,
  Trash2,
  Wrench,
} from "lucide-react";
import { app, openExternal } from "../lib/bridge";
import { toErrorMessage } from "../lib/errors";
import { MotionUnfold } from "./MotionUnfold";
import { useT } from "../lib/i18n";
import type { CapabilitiesView, MCPCatalogEntry, MCPServerInput, ServerView, SkillView } from "../lib/types";

type ExtensionsSection = "skills" | "mcp-browse" | "mcp-installed";

const EXTERNAL_SKILLS_MARKETPLACE_URL = "https://skills.sh";

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

function skillScopeLabel(scope: string, t: ReturnType<typeof useT>): string {
  switch (scope) {
    case "builtin":
      return t("caps.skillScopeBuiltin");
    case "project":
      return t("caps.skillScopeProject");
    case "custom":
      return t("caps.skillScopeCustom");
    case "global":
      return t("caps.skillScopeGlobal");
    default:
      return scope;
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
  const [section, setSection] = useState<ExtensionsSection>("skills");
  const [catalog, setCatalog] = useState<MCPCatalogEntry[]>([]);
  const [caps, setCaps] = useState<CapabilitiesView | null>(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState<string>("all");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [expandedServers, setExpandedServers] = useState<Set<string>>(new Set());
  const [expandedSkills, setExpandedSkills] = useState<Set<string>>(new Set());
  const [sourcesOpen, setSourcesOpen] = useState(false);

  const reload = useCallback(async () => {
    const [catalogItems, capabilities] = await Promise.all([
      app.ListMCPCatalog().catch(() => [] as MCPCatalogEntry[]),
      app.Capabilities().catch(() => ({ servers: [], skills: [], skillRoots: [] })),
    ]);
    setCatalog(catalogItems);
    setCaps(capabilities);
  }, []);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoading(true);
      try {
        await reload();
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [reload]);

  useEffect(() => {
    setQuery("");
    setCategory("all");
  }, [section]);

  const servers = caps?.servers ?? [];
  const skills = caps?.skills ?? [];
  const skillRoots = caps?.skillRoots ?? [];

  const installedNames = useMemo(() => new Set(servers.map((s) => s.name)), [servers]);

  const categories = useMemo(() => {
    const set = new Set(catalog.map((entry) => entry.category).filter(Boolean));
    return ["all", ...Array.from(set).sort((a, b) => a.localeCompare(b))];
  }, [catalog]);

  const stats = useMemo(() => {
    const connected = servers.filter((s) => s.status === "connected").length;
    const failed = servers.filter((s) => s.status === "failed").length;
    const enabledSkills = skills.filter((s) => s.enabled).length;
    return { connected, failed, installed: servers.length, enabledSkills, catalog: catalog.length };
  }, [catalog.length, servers, skills]);

  const filteredCatalog = useMemo(() => {
    const q = query.trim().toLowerCase();
    return catalog.filter((entry) => {
      if (category !== "all" && entry.category !== category) return false;
      if (!q) return true;
      return [entry.name, entry.category, entry.description, entry.transport].join(" ").toLowerCase().includes(q);
    });
  }, [catalog, category, query]);

  const filteredServers = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return servers;
    return servers.filter((s) =>
      [s.name, s.transport, s.status, s.error ?? "", s.tier ?? ""].join(" ").toLowerCase().includes(q),
    );
  }, [query, servers]);

  const filteredSkills = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return skills;
    return skills.filter((s) => [s.name, s.description, s.scope, s.runAs].join(" ").toLowerCase().includes(q));
  }, [query, skills]);

  const searchPlaceholder = useMemo(() => {
    switch (section) {
      case "skills":
        return t("extensions.searchSkills");
      case "mcp-browse":
        return t("extensions.searchMcp");
      case "mcp-installed":
        return t("extensions.searchInstalled");
    }
  }, [section, t]);

  const run = async (fn: () => Promise<void>, success?: string) => {
    setBusy(true);
    setErr(null);
    setNotice(null);
    try {
      await fn();
      await reload();
      if (success) setNotice(success);
    } catch (e) {
      setErr(toErrorMessage(e));
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

  const addSkillPath = () =>
    run(async () => {
      const path = await app.PickSkillFolder();
      if (path) await app.AddSkillPath(path);
    });

  const removeSkillPath = (path: string) =>
    run(async () => {
      await app.RemoveSkillPath(path);
    });

  const refreshSkills = () =>
    run(async () => {
      await app.RefreshSkills();
    });

  const toggleExpanded = (name: string) => {
    setExpandedServers((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  const toggleSkillExpanded = (name: string) => {
    setExpandedSkills((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  const navItems: { id: ExtensionsSection; label: string; count?: number; warn?: boolean }[] = [
    { id: "skills", label: t("extensions.nav.skills"), count: stats.enabledSkills },
    { id: "mcp-browse", label: t("extensions.nav.mcpBrowse"), count: stats.catalog },
    { id: "mcp-installed", label: t("extensions.nav.mcpInstalled"), count: stats.connected, warn: stats.failed > 0 },
  ];

  return (
    <div className="mode-center mode-center--plugins extensions-studio-shell">
      <aside className="extensions-studio__sidebar">
        <div className="extensions-studio__sidebar-head">
          <div className="extensions-studio__sidebar-title">
            <Sparkles size={16} />
            {t("extensions.title")}
          </div>
          <p className="extensions-studio__sidebar-sub">{t("extensions.subtitle")}</p>
        </div>

        <nav className="extensions-studio__nav" aria-label={t("extensions.title")}>
          <div className="extensions-studio__nav-group">
            <span className="extensions-studio__nav-label">{t("extensions.nav.skillsSection")}</span>
            <button
              type="button"
              className={`extensions-studio__nav-item${section === "skills" ? " extensions-studio__nav-item--active" : ""}`}
              onClick={() => setSection("skills")}
            >
              <Sparkles size={14} />
              <span>{t("extensions.nav.skills")}</span>
              <span className="extensions-studio__nav-count">{skills.length}</span>
            </button>
          </div>

          <div className="extensions-studio__nav-group">
            <span className="extensions-studio__nav-label">{t("extensions.nav.mcpSection")}</span>
            {navItems.slice(1).map((item) => (
              <button
                key={item.id}
                type="button"
                className={`extensions-studio__nav-item${section === item.id ? " extensions-studio__nav-item--active" : ""}`}
                onClick={() => setSection(item.id)}
              >
                {item.id === "mcp-browse" ? <Plus size={14} /> : <Plug size={14} />}
                <span>{item.label}</span>
                <span className={`extensions-studio__nav-count${item.warn ? " extensions-studio__nav-count--warn" : ""}`}>
                  {item.count ?? 0}
                </span>
              </button>
            ))}
          </div>
        </nav>

        <div className="extensions-studio__sidebar-foot">
          <p>{t("extensions.skillsMarketHint")}</p>
          <button type="button" className="extensions-studio__market-btn" onClick={() => openExternal(EXTERNAL_SKILLS_MARKETPLACE_URL)}>
            <ExternalLink size={14} />
            {t("plugins.openSkillsMarket")}
          </button>
        </div>
      </aside>

      <main className="extensions-studio__main">
        <header className="extensions-studio__toolbar">
          <div className="extensions-studio__toolbar-copy">
            <h2 className="extensions-studio__panel-title">
              {section === "skills"
                ? t("extensions.nav.skills")
                : section === "mcp-browse"
                  ? t("extensions.nav.mcpBrowse")
                  : t("extensions.nav.mcpInstalled")}
            </h2>
            <p className="extensions-studio__panel-sub">
              {section === "skills"
                ? t("extensions.skillsPanelHint")
                : section === "mcp-browse"
                  ? t("extensions.mcpBrowseHint")
                  : t("extensions.mcpInstalledHint", { connected: stats.connected, total: stats.installed })}
            </p>
          </div>
          <div className="extensions-studio__toolbar-actions">
            {section === "skills" ? (
              <>
                <button type="button" className="extensions-studio__action" disabled={busy} onClick={() => void refreshSkills()}>
                  <RefreshCw size={14} className={busy ? "dock-panel__spin" : undefined} />
                  {t("plugins.refreshSkills")}
                </button>
                <button type="button" className="extensions-studio__action extensions-studio__action--primary" disabled={busy} onClick={() => void addSkillPath()}>
                  <FolderPlus size={14} />
                  {t("plugins.addSkillPath")}
                </button>
              </>
            ) : (
              <button type="button" className="extensions-studio__action" disabled={busy || loading} onClick={() => void reload()}>
                <RefreshCw size={14} className={busy ? "dock-panel__spin" : undefined} />
                {t("plugins.refresh")}
              </button>
            )}
          </div>
        </header>

        <label className="extensions-studio__search">
          <Search size={15} />
          <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder={searchPlaceholder} />
        </label>

        {err ? <div className="extensions-studio__banner extensions-studio__banner--error">{err}</div> : null}
        {notice ? <div className="extensions-studio__banner extensions-studio__banner--ok">{notice}</div> : null}

        <div className="extensions-studio__scroll">
        {loading ? (
          <div className="extensions-studio__loading">
            <Loader2 size={18} className="dock-panel__spin" />
            <span>{t("plugins.loading")}</span>
          </div>
        ) : null}

        {!loading && section === "skills" ? (
          <div className="extensions-studio__skills">
            {skillRoots.length > 0 ? (
              <section className="extensions-studio__sources">
                <button type="button" className="extensions-studio__sources-toggle" onClick={() => setSourcesOpen((open) => !open)}>
                  <ChevronDown size={14} className={`extensions-studio__sources-chevron${sourcesOpen ? " extensions-studio__sources-chevron--open" : ""}`} />
                  <span>{t("plugins.skillSources")}</span>
                  <span className="extensions-studio__sources-count">{skillRoots.length}</span>
                </button>
                <MotionUnfold open={sourcesOpen}>
                  <div className="extensions-studio__sources-body">
                      {skillRoots.map((root) => (
                        <article key={`${root.scope}-${root.dir}`} className="extensions-studio__root">
                          <div className="extensions-studio__root-head">
                            <strong>{skillScopeLabel(root.scope, t)}</strong>
                            {root.scope === "custom" ? (
                              <button type="button" className="extensions-studio__icon-danger" disabled={busy} onClick={() => void removeSkillPath(root.dir)} aria-label={t("plugins.remove")}>
                                <Trash2 size={13} />
                              </button>
                            ) : null}
                          </div>
                          <code className="extensions-studio__root-path">{root.dir}</code>
                          <div className="extensions-studio__root-meta">
                            <span className="extensions-studio__chip">{t("plugins.rootSkills", { n: root.skills })}</span>
                            {root.status !== "ok" ? <span className="extensions-studio__error">{root.status}</span> : null}
                            {root.warning ? <span className="extensions-studio__error">{root.warning}</span> : null}
                          </div>
                        </article>
                      ))}
                  </div>
                </MotionUnfold>
              </section>
            ) : null}

            <div className="extensions-studio__skill-list">
              {filteredSkills.map((skill) => {
                const skillExpanded = expandedSkills.has(skill.name);
                const description = (skill.description ?? "").trim();
                const canExpand = description.length > 48;
                return (
                <article key={skill.name} className={`extensions-studio__skill${skill.enabled ? "" : " extensions-studio__skill--disabled"}`}>
                  <div className="extensions-studio__skill-row">
                    <div className="extensions-studio__skill-copy">
                      <div className="extensions-studio__skill-head">
                        <strong>/{skill.name}</strong>
                        <span className="extensions-studio__chip">{skillScopeLabel(skill.scope, t)}</span>
                        <span className="extensions-studio__chip extensions-studio__chip--muted">{skill.runAs}</span>
                      </div>
                      {!skillExpanded ? (
                        <p className="extensions-studio__skill-desc-line">{description || "—"}</p>
                      ) : null}
                    </div>
                    <div className="extensions-studio__skill-aside">
                      {canExpand ? (
                        <button
                          type="button"
                          className={`extensions-studio__skill-chevron${skillExpanded ? " extensions-studio__skill-chevron--open" : ""}`}
                          aria-expanded={skillExpanded}
                          aria-label={skillExpanded ? t("common.collapse") : t("common.expand")}
                          onClick={() => toggleSkillExpanded(skill.name)}
                        >
                          <ChevronDown size={14} />
                        </button>
                      ) : null}
                      <button type="button" className="extensions-studio__skill-toggle" disabled={busy} onClick={() => void toggleSkill(skill)}>
                        {skill.enabled ? t("plugins.disable") : t("plugins.enable")}
                      </button>
                    </div>
                  </div>
                  {canExpand ? (
                    <MotionUnfold open={skillExpanded}>
                      <p className="extensions-studio__skill-desc-full">{description}</p>
                    </MotionUnfold>
                  ) : null}
                </article>
              )})}
              {!filteredSkills.length ? (
                <div className="extensions-studio__empty">
                  <p>{t("plugins.emptySkills")}</p>
                  <button type="button" className="extensions-studio__market-btn" onClick={() => openExternal(EXTERNAL_SKILLS_MARKETPLACE_URL)}>
                    <ExternalLink size={14} />
                    {t("plugins.openSkillsMarket")}
                  </button>
                </div>
              ) : null}
            </div>
          </div>
        ) : null}

        {!loading && section === "mcp-browse" ? (
          <>
            {categories.length > 1 ? (
              <div className="extensions-studio__categories">
                {categories.map((item) => (
                  <button
                    key={item}
                    type="button"
                    className={`extensions-studio__category${category === item ? " extensions-studio__category--active" : ""}`}
                    onClick={() => setCategory(item)}
                  >
                    {item === "all" ? t("plugins.categoryAll") : item}
                  </button>
                ))}
              </div>
            ) : null}
            <div className="extensions-studio__grid">
              {filteredCatalog.map((entry) => {
                const installed = installedNames.has(entry.id) || installedNames.has(entry.name.toLowerCase());
                return (
                  <article key={entry.id} className="extensions-studio__card">
                    <div className="extensions-studio__card-head">
                      <strong>{entry.name}</strong>
                      <span className="extensions-studio__chip">{entry.category}</span>
                    </div>
                    <p className="extensions-studio__card-desc">{entry.description}</p>
                    <div className="extensions-studio__card-meta">
                      <span className="extensions-studio__chip extensions-studio__chip--muted">{entry.transport}</span>
                      {entry.tier ? <span className="extensions-studio__chip extensions-studio__chip--muted">{entry.tier}</span> : null}
                      {entry.official ? (
                        <span className="extensions-studio__official">
                          <ShieldCheck size={13} /> {t("plugins.official")}
                        </span>
                      ) : null}
                    </div>
                    <button
                      type="button"
                      className={`extensions-studio__install${installed ? " extensions-studio__install--done" : ""}`}
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
              {!filteredCatalog.length ? (
                <div className="extensions-studio__empty">
                  <p>{t("plugins.emptyMarketplace")}</p>
                </div>
              ) : null}
            </div>
          </>
        ) : null}

        {!loading && section === "mcp-installed" ? (
          <div className="extensions-studio__list">
            {filteredServers.map((server) => {
              const expanded = expandedServers.has(server.name);
              const endpoint =
                server.transport === "stdio"
                  ? [server.command, ...(server.args ?? [])].filter(Boolean).join(" ")
                  : server.url ?? "";
              return (
                <article key={server.name} className={`extensions-studio__row${expanded ? " extensions-studio__row--open" : ""}`}>
                  <div className="extensions-studio__row-main">
                    <button type="button" className="extensions-studio__row-toggle" onClick={() => toggleExpanded(server.name)}>
                      {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
                    </button>
                    <div className="extensions-studio__row-copy">
                      <div className="extensions-studio__row-title">
                        <strong>{server.name}</strong>
                        {server.builtIn ? <span className="extensions-studio__chip">{t("plugins.builtIn")}</span> : null}
                        <span className="extensions-studio__chip extensions-studio__chip--muted">{server.transport}</span>
                        {server.tier ? <span className="extensions-studio__chip extensions-studio__chip--muted">{server.tier}</span> : null}
                        <span className={`extensions-studio__status extensions-studio__status--${server.status}`}>
                          {serverStatusLabel(server.status, t)}
                        </span>
                      </div>
                      <div className="extensions-studio__row-meta">
                        {server.tools ? <span>{t("plugins.tools", { n: server.tools })}</span> : null}
                        {server.prompts ? <span>{t("plugins.prompts", { n: server.prompts })}</span> : null}
                        {server.resources ? <span>{t("plugins.resources", { n: server.resources })}</span> : null}
                        {server.error ? <span className="extensions-studio__error">{server.error}</span> : null}
                      </div>
                    </div>
                    <div className="extensions-studio__row-actions">
                      {server.status === "failed" ? (
                        <button type="button" disabled={busy} onClick={() => void retryServer(server.name)}>
                          {t("plugins.retry")}
                        </button>
                      ) : null}
                      <button
                        type="button"
                        disabled={busy || server.builtIn}
                        onClick={() => void toggleServer(server.name, server.status === "disabled")}
                      >
                        {server.status === "disabled" ? t("plugins.enable") : t("plugins.disable")}
                      </button>
                      {!server.builtIn ? (
                        <button type="button" className="extensions-studio__danger" disabled={busy} onClick={() => void removeServer(server.name)}>
                          {t("plugins.remove")}
                        </button>
                      ) : null}
                    </div>
                  </div>
                  <MotionUnfold open={expanded} className="extensions-studio__row-detail-wrap">
                    <div className="extensions-studio__row-detail">
                      {endpoint ? (
                        <div className="extensions-studio__detail-line">
                          <span>{t("plugins.endpoint")}</span>
                          <code>{endpoint}</code>
                        </div>
                      ) : null}
                      {server.toolList?.length ? (
                        <div className="extensions-studio__tools">
                          <div className="extensions-studio__tools-head">
                            <Wrench size={13} />
                            {t("plugins.toolsList")}
                          </div>
                          <ul>
                            {server.toolList.map((tool) => (
                              <li key={tool.name}>
                                <strong>{tool.name}</strong>
                                <span>{tool.description}</span>
                              </li>
                            ))}
                          </ul>
                        </div>
                      ) : null}
                    </div>
                  </MotionUnfold>
                </article>
              );
            })}
            {!filteredServers.length ? <div className="extensions-studio__empty">{t("plugins.emptyInstalled")}</div> : null}
          </div>
        ) : null}
        </div>
      </main>
    </div>
  );
}
