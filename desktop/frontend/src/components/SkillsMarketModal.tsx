import { useCallback, useEffect, useRef, useState } from "react";
import { Download, ExternalLink, Loader2, Search } from "lucide-react";
import { app, openExternal } from "../lib/bridge";
import { toErrorMessage } from "../lib/errors";
import { useT } from "../lib/i18n";
import type { SkillsMarketEntry } from "../lib/types";
import { StudioCenterModal } from "./StudioCenterModal";

const SKILLS_SH_URL = "https://skills.sh";
const PAGE_SIZE = 48;

type InstallScope = "global" | "project";

function mergeSkills(existing: SkillsMarketEntry[], incoming: SkillsMarketEntry[]) {
  if (!incoming.length) return existing;
  const seen = new Set(existing.map((item) => item.id));
  const next = [...existing];
  for (const item of incoming) {
    if (seen.has(item.id)) continue;
    seen.add(item.id);
    next.push(item);
  }
  return next;
}

export function SkillsMarketModal({
  installedNames,
  onClose,
  onInstalled,
}: {
  installedNames: Set<string>;
  onClose: () => void;
  onInstalled: () => void;
}) {
  const t = useT();
  const [query, setQuery] = useState("");
  const [searchQuery, setSearchQuery] = useState("skill");
  const [skills, setSkills] = useState<SkillsMarketEntry[]>([]);
  const [page, setPage] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [scopePick, setScopePick] = useState<SkillsMarketEntry | null>(null);
  const [projectAvailable, setProjectAvailable] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const requestSeq = useRef(0);

  useEffect(() => {
    const trimmed = query.trim();
    const timer = window.setTimeout(() => {
      setSearchQuery(trimmed || "skill");
    }, trimmed ? 280 : 0);
    return () => window.clearTimeout(timer);
  }, [query]);

  const fetchPage = useCallback(async (baseQuery: string, nextPage: number, append: boolean) => {
    const seq = ++requestSeq.current;
    if (append) {
      setLoadingMore(true);
    } else {
      setLoading(true);
      setErr(null);
    }
    try {
      const [search, canProject] = await Promise.all([
        app.SearchSkillsMarket(baseQuery, PAGE_SIZE, nextPage),
        append ? Promise.resolve(projectAvailable) : app.SkillsMarketProjectInstallAvailable(),
      ]);
      if (seq !== requestSeq.current) return;
      setSkills((prev) => (append ? mergeSkills(prev, search.skills ?? []) : search.skills ?? []));
      setPage(nextPage);
      setHasMore(Boolean(search.hasMore));
      if (!append) setProjectAvailable(canProject);
    } catch (e) {
      if (seq !== requestSeq.current) return;
      setErr(toErrorMessage(e));
      if (!append) setSkills([]);
      setHasMore(false);
    } finally {
      if (seq !== requestSeq.current) return;
      if (append) setLoadingMore(false);
      else setLoading(false);
    }
  }, [projectAvailable]);

  useEffect(() => {
    setPage(0);
    setHasMore(false);
    void fetchPage(searchQuery, 0, false);
  }, [fetchPage, searchQuery]);

  const loadMore = () => {
    if (loading || loadingMore || !hasMore) return;
    void fetchPage(searchQuery, page + 1, true);
  };

  const install = async (entry: SkillsMarketEntry, scope: InstallScope) => {
    setBusyId(entry.id);
    setErr(null);
    setNotice(null);
    try {
      const result = await app.InstallSkillsMarketSkill(entry.source, entry.skillId, scope);
      setNotice(t("skillsMarket.installedOk", { name: result.name, scope: t(scope === "global" ? "skillsMarket.scopeGlobal" : "skillsMarket.scopeProject") }));
      setScopePick(null);
      onInstalled();
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusyId(null);
    }
  };

  return (
    <StudioCenterModal
      title={t("skillsMarket.title")}
      titleId="skills-market-title"
      onClose={onClose}
      wide
      className="skills-market-modal"
    >
      <div className="skills-market">
        <div className="skills-market__toolbar">
          <label className="skills-market__search">
            <Search size={15} />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("skillsMarket.searchPlaceholder")}
              autoFocus
            />
          </label>
          <button type="button" className="skills-market__external" onClick={() => openExternal(SKILLS_SH_URL)}>
            <ExternalLink size={14} />
            {t("skillsMarket.openWebsite")}
          </button>
        </div>

        <p className="skills-market__hint">{t("skillsMarket.hint")}</p>

        {err ? <div className="skills-market__banner skills-market__banner--error">{err}</div> : null}
        {notice ? <div className="skills-market__banner skills-market__banner--ok">{notice}</div> : null}

        <div className="skills-market__list" aria-busy={loading || loadingMore}>
          {loading && !skills.length ? (
            <div className="skills-market__loading">
              <Loader2 size={18} className="dock-panel__spin" />
              <span>{t("skillsMarket.loading")}</span>
            </div>
          ) : null}

          {skills.map((entry) => {
            const installing = busyId === entry.id;
            return (
              <article key={entry.id} className="skills-market__card">
                <div className="skills-market__card-copy">
                  <div className="skills-market__card-head">
                    <strong>{entry.name}</strong>
                    <span className="skills-market__chip">{entry.source}</span>
                  </div>
                  <div className="skills-market__card-meta">
                    <span>{t("skillsMarket.installs", { n: entry.installs.toLocaleString() })}</span>
                  </div>
                </div>
                <button
                  type="button"
                  className={`skills-market__install${installedNames.has(entry.name) ? " skills-market__install--done" : ""}`}
                  disabled={installing || installedNames.has(entry.name)}
                  onClick={() => setScopePick(entry)}
                >
                  {installing ? <Loader2 size={14} className="dock-panel__spin" /> : <Download size={14} />}
                  {installedNames.has(entry.name) ? t("plugins.installed") : t("plugins.install")}
                </button>
              </article>
            );
          })}

          {!loading && !skills.length ? <div className="skills-market__empty">{t("skillsMarket.empty")}</div> : null}
        </div>

        {hasMore ? (
          <div className="skills-market__footer">
            <button type="button" className="skills-market__load-more" disabled={loading || loadingMore} onClick={loadMore}>
              {loadingMore ? <Loader2 size={14} className="dock-panel__spin" /> : null}
              {loadingMore ? t("skillsMarket.loadingMore") : t("skillsMarket.loadMore")}
            </button>
          </div>
        ) : null}
      </div>

      {scopePick ? (
        <div className="skills-market-scope" role="presentation" onMouseDown={(event) => {
          if (event.target === event.currentTarget) setScopePick(null);
        }}>
          <div className="skills-market-scope__panel" role="dialog" aria-modal="true" aria-labelledby="skills-market-scope-title" onMouseDown={(e) => e.stopPropagation()}>
            <h4 id="skills-market-scope-title" className="skills-market-scope__title">
              {t("skillsMarket.scopeTitle", { name: scopePick.name })}
            </h4>
            <p className="skills-market-scope__hint">{t("skillsMarket.scopeHint")}</p>
            <div className="skills-market-scope__actions">
              <button type="button" className="btn btn--small btn--primary" disabled={busyId === scopePick.id} onClick={() => void install(scopePick, "global")}>
                {t("skillsMarket.installGlobal")}
              </button>
              <button
                type="button"
                className="btn btn--small"
                disabled={!projectAvailable || busyId === scopePick.id}
                title={projectAvailable ? undefined : t("skillsMarket.projectUnavailable")}
                onClick={() => void install(scopePick, "project")}
              >
                {t("skillsMarket.installProject")}
              </button>
              <button type="button" className="btn btn--small" onClick={() => setScopePick(null)}>
                {t("common.cancel")}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </StudioCenterModal>
  );
}
