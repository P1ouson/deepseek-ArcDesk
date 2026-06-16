import { useCallback, useEffect, useState } from "react";
import { ArrowLeft, GitBranch } from "lucide-react";
import { asArray } from "../lib/array";
import { app } from "../lib/bridge";
import { formatMoney, formatTokens } from "../lib/formatMoney";
import { sessionCacheRate, stepCacheRate } from "../lib/cacheRate";
import { estimatePromptCacheSavingsPct } from "../lib/cacheEconomy";
import { useT, type Translator } from "../lib/i18n";
import type { DictKey } from "../locales/en";
import { useWorkspaceChanges } from "../lib/useWorkspaceChanges";
import type { BalanceInfo, ContextInfo, ContextPanelInfo, EffortInfo, Mode, WireUsage, WorkspaceChangeView } from "../lib/types";

interface ContextPanelProps {
  tabId?: string;
  context?: ContextInfo;
  usage?: WireUsage;
  sessionCost?: number;
  sessionCurrency?: string;
  scopeLabel?: string;
  refreshKey?: number;
  modelLabel?: string;
  mode?: Mode;
  effort?: EffortInfo;
  balance?: BalanceInfo;
  running?: boolean;
  cwd?: string;
  onOpenChangesTab?: () => void;
  onOpenGitTab?: () => void;
}

type ContextDetail = "read" | "changed";

function fmtTime(ms?: number): string {
  if (!ms) return "";
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

interface HealthResult {
  tone: "good" | "notice" | "warn";
  labelKey: DictKey;
  bodyKey: DictKey;
  vars: Record<string, string | number>;
}

function contextHealth(usagePct: number, cachePct: number, readCount: number): HealthResult {
  if (usagePct >= 85) {
    return {
      tone: "warn",
      labelKey: "context.healthNearLimit",
      bodyKey: "context.healthNearLimitBody",
      vars: { pct: usagePct },
    };
  }
  if (readCount >= 8) {
    return {
      tone: "notice",
      labelKey: "context.healthManyFiles",
      bodyKey: "context.healthManyFilesBody",
      vars: { count: readCount },
    };
  }
  if (cachePct > 0 && cachePct < 50) {
    return {
      tone: "notice",
      labelKey: "context.healthLowCache",
      bodyKey: "context.healthLowCacheBody",
      vars: { pct: cachePct },
    };
  }
  return {
    tone: "good",
    labelKey: "context.healthGood",
    bodyKey: "context.healthGoodBody",
    vars: {},
  };
}

function fmtPct(pct: string | null): string {
  return pct != null ? `${pct}%` : "—";
}

function TokenUsageCard({ usage, info }: { usage?: WireUsage; info: ContextPanelInfo | null }) {
  const t = useT();
  const prompt = usage?.promptTokens ?? info?.promptTokens ?? 0;
  const completion = usage?.completionTokens ?? info?.completionTokens ?? 0;
  const reasoning = usage?.reasoningTokens ?? info?.reasoningTokens ?? 0;
  const total = usage?.totalTokens ?? prompt + completion + reasoning;
  const stepHit = usage?.cacheHitTokens ?? info?.cacheHitTokens ?? 0;
  const stepMiss = usage?.cacheMissTokens ?? info?.cacheMissTokens ?? 0;
  const sessHit = usage?.sessionCacheHitTokens ?? 0;
  const sessMiss = usage?.sessionCacheMissTokens ?? 0;
  const stepDenom = stepHit + stepMiss;
  const sessDenom = sessHit + sessMiss;
  const hasUsage = total > 0 || prompt > 0 || completion > 0;
  const hasCache = stepDenom > 0 || sessDenom > 0;
  const stepPct = stepCacheRate(usage) ?? (stepDenom > 0 ? ((stepHit / stepDenom) * 100).toFixed(1) : null);
  const sessionPct = sessionCacheRate(usage) ?? (sessDenom > 0 ? ((sessHit / sessDenom) * 100).toFixed(1) : null);
  const stepBarHit = stepDenom > 0 ? Math.round((stepHit / stepDenom) * 100) : 0;
  const cacheTokensForSavings = sessDenom > 0 ? { hit: sessHit, miss: sessMiss } : stepDenom > 0 ? { hit: stepHit, miss: stepMiss } : null;
  const savingsPct = cacheTokensForSavings
    ? estimatePromptCacheSavingsPct(cacheTokensForSavings.hit, cacheTokensForSavings.miss)
    : null;
  const reuseCalls = usage?.toolReuse?.sessionCalls ?? info?.toolReuseCalls ?? 0;
  const reuseDupes = usage?.toolReuse?.sessionDuplicates ?? info?.toolReuseDuplicates ?? 0;
  const reuseCacheDupes = usage?.toolReuse?.sessionCacheableDupes ?? info?.toolReuseCacheableDupes ?? 0;
  const reuseCacheHits = usage?.toolReuse?.sessionCacheHits ?? info?.toolReuseCacheHits ?? 0;
  const hasToolReuse = reuseCalls > 0;
  const reusePct = reuseCalls > 0 ? ((reuseDupes / reuseCalls) * 100).toFixed(1) : null;
  const reuseCachePct =
    (usage?.toolReuse?.sessionCacheableCalls ?? 0) > 0
      ? ((reuseCacheDupes / (usage?.toolReuse?.sessionCacheableCalls ?? 1)) * 100).toFixed(1)
      : null;

  const rows: Array<{ label: string; value: string }> = [
    { label: t("context.prompt"), value: hasUsage ? formatTokens(prompt) : "—" },
    { label: t("context.completion"), value: hasUsage ? formatTokens(completion) : "—" },
  ];
  if (reasoning > 0) {
    rows.push({ label: t("context.reasoning"), value: formatTokens(reasoning) });
  }
  rows.push({ label: t("context.total"), value: hasUsage ? formatTokens(total) : "—" });

  return (
    <section className="dock-panel__card context-panel__tokens">
      <header className="context-panel__tokens-head">
        <h3 className="context-panel__tokens-title">{t("context.tokenUsage")}</h3>
        <span className="context-panel__tokens-sub">{t("context.latestRequest")}</span>
      </header>

      <div className="context-panel__token-grid">
        {rows.map((row) => (
          <div key={row.label} className="context-panel__token-row">
            <span>{row.label}</span>
            <strong>{row.value}</strong>
          </div>
        ))}
      </div>

      <div className="context-panel__tokens-divider" />

      <header className="context-panel__tokens-head context-panel__tokens-head--cache">
        <h3 className="context-panel__tokens-title">{t("context.cacheTokens")}</h3>
      </header>

      {!hasCache ? (
        <p className="context-panel__tokens-empty">{t("context.noUsageYet")}</p>
      ) : (
        <>
          {stepDenom > 0 && (
            <div
              className="context-panel__cache-bar"
              role="img"
              aria-label={`${t("context.cacheStepRate")} ${stepPct ?? "0"}%`}
            >
              <div className="context-panel__cache-bar-hit" style={{ width: `${stepBarHit}%` }} />
              <div className="context-panel__cache-bar-miss" style={{ width: `${100 - stepBarHit}%` }} />
            </div>
          )}

          <div className="context-panel__token-grid">
            <div className="context-panel__token-row">
              <span className="context-panel__token-label context-panel__token-label--hit">{t("context.cacheHitTokens")}</span>
              <strong>{formatTokens(stepDenom > 0 ? stepHit : sessHit)}</strong>
            </div>
            <div className="context-panel__token-row">
              <span className="context-panel__token-label context-panel__token-label--miss">{t("context.cacheMissTokens")}</span>
              <strong>{formatTokens(stepDenom > 0 ? stepMiss : sessMiss)}</strong>
            </div>
            {stepDenom > 0 && (
              <div className="context-panel__token-row">
                <span>{t("context.cacheStepRate")}</span>
                <strong>{fmtPct(stepPct)}</strong>
              </div>
            )}
            {sessDenom > 0 && (
              <>
                <div className="context-panel__token-row">
                  <span>{t("context.cacheSessionRate")}</span>
                  <strong>{fmtPct(sessionPct)}</strong>
                </div>
                <div className="context-panel__token-row context-panel__token-row--muted">
                  <span>{t("context.cacheSessionTokens")}</span>
                  <strong>
                    {formatTokens(sessHit)} / {formatTokens(sessDenom)}
                  </strong>
                </div>
              </>
            )}
            {savingsPct != null && savingsPct > 0 && (
              <p className="context-panel__cache-savings">
                {t("context.cacheSavingsEstimate", { pct: savingsPct })}
              </p>
            )}
          </div>
        </>
      )}

      {hasToolReuse && (
        <>
          <div className="context-panel__tokens-divider" />
          <header className="context-panel__tokens-head context-panel__tokens-head--cache">
            <h3 className="context-panel__tokens-title">{t("context.toolReuseTitle")}</h3>
          </header>
          <div className="context-panel__token-grid">
            <div className="context-panel__token-row">
              <span>{t("context.toolReuseCalls")}</span>
              <strong>{formatTokens(reuseCalls)}</strong>
            </div>
            <div className="context-panel__token-row">
              <span>{t("context.toolReuseDuplicates")}</span>
              <strong>{formatTokens(reuseDupes)}</strong>
            </div>
            {reusePct != null && (
              <div className="context-panel__token-row">
                <span>{t("context.toolReuseDuplicateRate")}</span>
                <strong>{fmtPct(reusePct)}</strong>
              </div>
            )}
            {reuseCacheDupes > 0 && reuseCachePct != null && (
              <div className="context-panel__token-row context-panel__token-row--muted">
                <span>{t("context.toolReuseCacheableDupes")}</span>
                <strong>{fmtPct(reuseCachePct)}</strong>
              </div>
            )}
            {reuseCacheHits > 0 && (
              <div className="context-panel__token-row context-panel__token-row--muted">
                <span>{t("context.toolReuseCacheHits")}</span>
                <strong>{formatTokens(reuseCacheHits)}</strong>
              </div>
            )}
          </div>
          <p className="context-panel__cache-savings">{t("context.toolReuseHint")}</p>
        </>
      )}
    </section>
  );
}

function modeLabel(mode: Mode | undefined, t: Translator): string {
  if (mode === "plan") return t("composer.modePlan");
  if (mode === "yolo") return t("composer.modeYolo");
  return t("composer.modeNormal");
}

interface GitOverviewStats {
  total: number;
  staged: number;
  unstaged: number;
  untracked: number;
}

function summarizeGitChanges(files: WorkspaceChangeView[]): GitOverviewStats {
  const gitFiles = files.filter((row) => row.sources.includes("git"));
  let staged = 0;
  let unstaged = 0;
  let untracked = 0;
  for (const row of gitFiles) {
    const raw = (row.gitStatus ?? "").trim();
    if (!raw) continue;
    if (raw === "??" || raw.includes("?")) {
      untracked += 1;
      continue;
    }
    const index = raw.length >= 2 ? raw[0]! : " ";
    const work = raw.length >= 2 ? raw[1]! : raw[0]!;
    if (index !== " " && index !== "?") staged += 1;
    if (work !== " " && work !== "?") unstaged += 1;
  }
  return { total: gitFiles.length, staged, unstaged, untracked };
}

export function ContextPanel({
  tabId,
  context,
  usage,
  sessionCost,
  sessionCurrency,
  scopeLabel,
  refreshKey,
  modelLabel,
  mode,
  effort,
  balance,
  running = false,
  cwd,
  onOpenChangesTab,
  onOpenGitTab,
}: ContextPanelProps) {
  const t = useT();
  const [info, setInfo] = useState<ContextPanelInfo | null>(null);
  const [detailView, setDetailView] = useState<ContextDetail | null>(null);
  const [query, setQuery] = useState("");
  const [gitBranch, setGitBranch] = useState("");
  const { changes, loading: gitLoading } = useWorkspaceChanges(cwd, refreshKey);

  const refresh = useCallback(async () => {
    if (!tabId) return;
    try {
      setInfo(await app.ContextPanel(tabId));
    } catch {
      /* bridge unavailable */
    }
  }, [tabId]);

  useEffect(() => {
    const id = window.setInterval(() => void refresh(), 2000);
    return () => window.clearInterval(id);
  }, [refresh]);

  useEffect(() => {
    void refresh();
  }, [refresh, refreshKey]);

  useEffect(() => {
    if (!changes?.gitAvailable) {
      setGitBranch("");
      return;
    }
    let cancelled = false;
    void app.RunShellQuiet("git branch --show-current").then((result) => {
      if (cancelled || result.err) return;
      setGitBranch(result.output.trim());
    });
    return () => {
      cancelled = true;
    };
  }, [changes?.gitAvailable, cwd, refreshKey]);

  const usedTokens = context?.used && context.used > 0 ? context.used : info?.usedTokens ?? 0;
  const windowTokens = context?.window && context.window > 0 ? context.window : info?.windowTokens ?? 0;
  const cacheHitTokens = usage?.cacheHitTokens && usage.cacheHitTokens > 0 ? usage.cacheHitTokens : info?.cacheHitTokens ?? 0;
  const cacheMissTokens = usage?.cacheMissTokens && usage.cacheMissTokens > 0 ? usage.cacheMissTokens : info?.cacheMissTokens ?? 0;
  const cost = sessionCost && sessionCost > 0 ? sessionCost : info?.sessionCost ?? info?.sessionCostUsd ?? 0;
  const currency = sessionCurrency || info?.sessionCurrency || usage?.currency || "CNY";
  const readFiles = asArray(info?.readFiles);
  const changedFiles = asArray(info?.changedFiles);

  const usagePct = windowTokens > 0 ? Math.round((usedTokens / windowTokens) * 100) : 0;
  const cachePct = cacheHitTokens + cacheMissTokens > 0
    ? Math.round((cacheHitTokens / (cacheHitTokens + cacheMissTokens)) * 100)
    : 0;
  const readRows = readFiles.map((f, i) => ({
    key: `${f.path}-${i}`,
    path: f.path,
    meta: `#${f.turn}`,
    time: fmtTime(f.time),
    detail: f.limit ? `${f.offset ?? 0}-${(f.offset ?? 0) + f.limit}${f.truncated ? " truncated" : ""}` : "",
  }));
  const changedRows = changedFiles.map((f, i) => ({
    key: `${f.path}-${i}`,
    path: f.path,
    meta: f.gitStatus || asArray(f.sources).join(", ") || "changed",
    time: fmtTime(f.latestTime),
    detail: asArray(f.turns).length > 0 ? `T${asArray(f.turns).join(",")}` : "",
  }));
  const normalizedQuery = query.trim().toLowerCase();
  const filterRows = (rows: typeof readRows) => {
    if (!normalizedQuery) return rows;
    return rows.filter((row) =>
      `${row.path} ${row.meta} ${row.time} ${row.detail}`.toLowerCase().includes(normalizedQuery),
    );
  };
  const filteredReadRows = filterRows(readRows);
  const filteredChangedRows = filterRows(changedRows);
  const health = contextHealth(usagePct, cachePct, readRows.length);
  const detailRows = detailView === "changed" ? filteredChangedRows : filteredReadRows;
  const detailTitle = detailView === "changed" ? t("context.sessionChanges") : t("context.referencedFiles");
  const detailCount = detailView === "changed" ? changedRows.length : readRows.length;
  const detailEmpty = detailView === "changed" ? t("context.noChanges") : t("context.noReads");
  const detailPlaceholder = detailView === "changed" ? t("context.filterChanges") : t("context.filterReads");
  const detailNote = detailView === "changed"
    ? t("context.changedNote", { count: detailCount })
    : t("context.readNote", { count: detailCount });
  const billingAvailable = balance?.available === true;
  const balanceText = billingAvailable && balance.display ? balance.display : "--";
  const effortLevel = effort?.supported ? effort.current || "auto" : null;
  const gitStats = summarizeGitChanges(asArray(changes?.files));
  const gitPreviewRows = asArray(changes?.files)
    .filter((row) => row.sources.includes("git"))
    .slice(0, 3)
    .map((row, index) => ({
      key: `${row.path}-${index}`,
      path: row.path,
      meta: row.gitStatus?.trim() || "git",
      time: fmtTime(row.latestTime),
      detail: "",
    }));

  const openDetail = (next: ContextDetail) => {
    setDetailView(next);
    setQuery("");
  };

  const closeDetail = () => {
    setDetailView(null);
    setQuery("");
  };

  return (
    <div className="dock-panel context-panel">
      <header className="dock-panel__head">
        <div className="dock-panel__head-main">
          <h2 className="dock-panel__title">{detailView ? detailTitle : t("context.overview")}</h2>
          <p className="dock-panel__meta">{scopeLabel || t("context.scopeGlobal")}</p>
        </div>
        {detailView ? (
          <button type="button" className="dock-panel__text-btn context-panel__back" onClick={closeDetail}>
            <ArrowLeft size={13} strokeWidth={1.75} />
            {t("rightDock.overview")}
          </button>
        ) : null}
      </header>

      {detailView && (
        <div className="dock-panel__section-head">
          <p className="context-panel__detail-note">{detailNote}</p>
          <input
            className="dock-panel__filter"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={detailPlaceholder}
            aria-label={detailPlaceholder}
          />
        </div>
      )}

      <div className="context-panel__body">
        {detailView ? (
          <FileTable empty={detailEmpty} rows={detailRows} />
        ) : (
          <section className="context-panel__overview">
            {running && (
              <p className="context-panel__running" role="status">
                <span className="context-panel__running-dot" />
                {t("context.running")}
              </p>
            )}

            <div className="dock-panel__card context-panel__stats">
              <div className="context-panel__meter-head">
                <span>{t("context.windowUsage")}</span>
                <strong>{usagePct}%</strong>
              </div>
              <div className="context-panel__meter-track" aria-hidden="true">
                <div
                  className={`context-panel__meter-fill${usagePct >= 85 ? " context-panel__meter-fill--warn" : ""}`}
                  style={{ width: `${Math.min(100, Math.max(0, usagePct))}%` }}
                />
              </div>
              <div className="context-panel__meter-meta">
                {formatTokens(usedTokens)} / {formatTokens(windowTokens)} {t("status.tokens")}
              </div>

              <div className="context-panel__kv">
                <div className="context-panel__kv-row">
                  <span>{t("context.model")}</span>
                  <strong>{modelLabel || "—"}</strong>
                </div>
                <div className="context-panel__kv-row">
                  <span>{t("context.runMode")}</span>
                  <strong>{modeLabel(mode, t)}</strong>
                </div>
                {effortLevel && (
                  <div className="context-panel__kv-row">
                    <span>{t("context.effort")}</span>
                    <strong>{effortLevel}</strong>
                  </div>
                )}
                <div className="context-panel__kv-row">
                  <span>{t("context.sessionCost")}</span>
                  <strong>{billingAvailable ? formatMoney(cost, currency) : "--"}</strong>
                </div>
                <div className="context-panel__kv-row">
                  <span>{t("status.balanceTitle")}</span>
                  <strong>{balanceText}</strong>
                </div>
              </div>
            </div>

            <TokenUsageCard usage={usage} info={info} />

            <GitOverviewCard
              loading={gitLoading}
              gitAvailable={changes?.gitAvailable}
              gitErr={changes?.gitErr}
              branch={gitBranch}
              stats={gitStats}
              rows={gitPreviewRows}
              onOpenGit={onOpenGitTab}
            />

            <p className={`context-panel__health context-panel__health--${health.tone}`}>
              <span className="context-panel__health-dot" />
              <span>{t(health.labelKey, health.vars)}</span>
              {health.tone !== "good" && <small>{t(health.bodyKey, health.vars)}</small>}
            </p>

            <PreviewSection
              title={t("context.referencedFiles")}
              meta={t("context.readMeta", { count: readRows.length })}
              action={t("context.viewAll")}
              onAction={() => openDetail("read")}
              rows={readRows.slice(0, 3)}
              empty={t("context.noReads")}
            />
            <PreviewSection
              title={t("context.sessionChanges")}
              meta={t("context.changedMeta", { count: changedRows.length })}
              action={t("context.viewAll")}
              onAction={() => {
                if (onOpenChangesTab) onOpenChangesTab();
                else openDetail("changed");
              }}
              rows={changedRows.slice(0, 3)}
              empty={t("context.noChanges")}
            />
          </section>
        )}
      </div>
    </div>
  );
}

function GitOverviewCard({
  loading,
  gitAvailable,
  gitErr,
  branch,
  stats,
  rows,
  onOpenGit,
}: {
  loading: boolean;
  gitAvailable?: boolean;
  gitErr?: string;
  branch: string;
  stats: GitOverviewStats;
  rows: Array<{ key: string; path: string; meta: string; time: string; detail: string }>;
  onOpenGit?: () => void;
}) {
  const t = useT();

  if (gitAvailable === false) {
    return (
      <section className="dock-panel__card context-panel__git">
        <header className="context-panel__git-head">
          <div className="context-panel__git-title">
            <GitBranch size={14} />
            <span>{t("context.git")}</span>
          </div>
        </header>
        <p className="context-panel__git-note context-panel__git-note--warn">
          {gitErr ? gitErr : t("context.gitUnavailable")}
        </p>
      </section>
    );
  }

  const summary =
    stats.total === 0
      ? t("context.gitClean")
      : t("context.gitChanges", { count: stats.total });

  return (
    <section className="dock-panel__card context-panel__git">
      <header className="context-panel__git-head">
        <div className="context-panel__git-title">
          <GitBranch size={14} />
          <span>{t("context.git")}</span>
        </div>
        {onOpenGit && (
          <button type="button" className="dock-panel__text-btn" onClick={onOpenGit}>
            {t("context.openGit")}
          </button>
        )}
      </header>

      <div className="context-panel__git-branch">
        <span className="context-panel__git-branch-label">{t("context.gitBranch")}</span>
        <strong>{loading && !branch ? "…" : branch || "—"}</strong>
      </div>

      <p className="context-panel__git-note">{summary}</p>

      {stats.total > 0 && (
        <div className="context-panel__git-stats">
          {stats.staged > 0 && (
            <span className="context-panel__git-pill context-panel__git-pill--staged">
              {t("context.gitStaged", { count: stats.staged })}
            </span>
          )}
          {stats.unstaged > 0 && (
            <span className="context-panel__git-pill context-panel__git-pill--unstaged">
              {t("context.gitUnstaged", { count: stats.unstaged })}
            </span>
          )}
          {stats.untracked > 0 && (
            <span className="context-panel__git-pill context-panel__git-pill--untracked">
              {t("context.gitUntracked", { count: stats.untracked })}
            </span>
          )}
        </div>
      )}

      {rows.length > 0 && <FileTable rows={rows} empty={t("git.empty")} compact />}
    </section>
  );
}

function PreviewSection({
  title,
  meta,
  action,
  onAction,
  rows,
  empty,
}: {
  title: string;
  meta?: string;
  action: string;
  onAction: () => void;
  rows: Array<{ key: string; path: string; meta: string; time: string; detail: string }>;
  empty: string;
}) {
  return (
    <section className="context-panel__preview">
      <header className="context-panel__preview-head">
        <div>
          <h3 className="context-panel__preview-title">{title}</h3>
          {meta && <span className="context-panel__preview-meta">{meta}</span>}
        </div>
        {rows.length > 0 && (
          <button type="button" className="dock-panel__text-btn" onClick={onAction}>
            {action}
          </button>
        )}
      </header>
      <FileTable rows={rows} empty={empty} compact />
    </section>
  );
}

function FileTable({
  rows,
  empty,
  compact = false,
}: {
  rows: Array<{ key: string; path: string; meta: string; time: string; detail: string }>;
  empty: string;
  compact?: boolean;
}) {
  if (rows.length === 0) {
    return (
      <p className={`dock-panel__empty${compact ? " dock-panel__empty--compact" : ""}`}>
        <span>{empty}</span>
      </p>
    );
  }
  return (
    <ul className={`dock-panel__list dock-panel__list--flush${compact ? " dock-panel__list--compact" : ""}`}>
      {rows.map((row) => (
        <li key={row.key}>
          <div className="dock-panel__row dock-panel__row--static">
            <span className="dock-panel__row-copy">
              <span className="dock-panel__row-name">{row.path}</span>
              {row.detail && <span className="dock-panel__row-detail">{row.detail}</span>}
            </span>
            <span className="dock-panel__row-tags">
              <span className="dock-panel__pill dock-panel__pill--muted">{row.meta}</span>
              {row.time && <span className="context-panel__time">{row.time}</span>}
            </span>
          </div>
        </li>
      ))}
    </ul>
  );
}
