import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, RefreshCw } from "lucide-react";
import { app } from "../lib/bridge";
import { formatMoney, formatTokens } from "../lib/formatMoney";
import { useT } from "../lib/i18n";
import { Tooltip } from "./Tooltip";
import {
  buildSampleUsageChart,
  buildUsageChart,
  hasUsageActivity,
  loadUsageActivity,
  summarizeUsageActivity,
  USAGE_ACTIVITY_UPDATE_EVENT,
  type UsageChartPoint,
} from "../lib/usageActivity";
import type { BalanceInfo, CapabilitiesView } from "../lib/types";

function UsageActivityChart({
  points,
  sample,
  currency,
}: {
  points: UsageChartPoint[];
  sample: boolean;
  currency: string;
}) {
  const t = useT();
  const [hoveredDate, setHoveredDate] = useState<string | null>(null);
  const max = Math.max(...points.map((p) => p.tokens), 1);
  const hovered = points.find((p) => p.date === hoveredDate) ?? null;

  return (
    <div className={`usage-chart${sample ? " usage-chart--sample" : ""}`}>
      <div className="usage-chart__plot" role="img" aria-label={t("settings.usage.chartAria")}>
        {hovered ? (
          <div className="usage-chart__tooltip" role="tooltip">
            <strong>{hovered.label}</strong>
            <span>{t("settings.usage.chartTipTokens", { value: formatTokens(hovered.tokens) })}</span>
            <span>{t("settings.usage.chartTipCost", { value: formatMoney(hovered.cost, currency) })}</span>
            <span>{t("settings.usage.chartTipTurns", { n: String(hovered.turns) })}</span>
          </div>
        ) : null}
        <div className="usage-chart__bars">
          {points.map((point) => {
            const height = point.tokens > 0 ? Math.max(8, Math.round((point.tokens / max) * 100)) : 4;
            const active = hoveredDate === point.date;
            return (
              <div
                key={point.date}
                className={`usage-chart__col${active ? " usage-chart__col--active" : ""}`}
                onMouseEnter={() => setHoveredDate(point.date)}
                onMouseLeave={() => setHoveredDate((cur) => (cur === point.date ? null : cur))}
                onFocus={() => setHoveredDate(point.date)}
                onBlur={() => setHoveredDate((cur) => (cur === point.date ? null : cur))}
              >
                <button type="button" className="usage-chart__hit" aria-label={t("settings.usage.chartDayAria", { date: point.label })}>
                  <div className="usage-chart__bar-wrap">
                    <div
                      className={`usage-chart__bar${point.tokens > 0 ? "" : " usage-chart__bar--empty"}`}
                      style={{ height: `${height}%` }}
                    />
                  </div>
                </button>
                <span className="usage-chart__label">{point.label}</span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function StatCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="usage-stat-card">
      <span className="usage-stat-card__label">{label}</span>
      <strong className="usage-stat-card__value">{value}</strong>
      {hint ? <span className="usage-stat-card__hint">{hint}</span> : null}
    </div>
  );
}

export function UsageInsightsSettings() {
  const t = useT();
  const [loading, setLoading] = useState(true);
  const [balance, setBalance] = useState<BalanceInfo | null>(null);
  const [caps, setCaps] = useState<CapabilitiesView | null>(null);
  const [buckets, setBuckets] = useState(() => loadUsageActivity());
  const currency = "CNY";

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [balanceRes, capsRes] = await Promise.all([
        app.Balance().catch(() => null),
        app.Capabilities().catch(() => ({ servers: [], skills: [], skillRoots: [] })),
      ]);
      setBalance(balanceRes);
      setCaps(capsRes);
      setBuckets(loadUsageActivity());
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
    const onUpdate = () => setBuckets(loadUsageActivity());
    window.addEventListener(USAGE_ACTIVITY_UPDATE_EVENT, onUpdate);
    return () => window.removeEventListener(USAGE_ACTIVITY_UPDATE_EVENT, onUpdate);
  }, [refresh]);

  const summary = useMemo(() => summarizeUsageActivity(buckets), [buckets]);
  const usingSample = !hasUsageActivity(buckets);
  const chartPoints = useMemo(
    () => (usingSample ? buildSampleUsageChart(14) : buildUsageChart(buckets, 14)),
    [buckets, usingSample],
  );
  const skillsEnabled = caps?.skills.filter((sk) => sk.enabled).length ?? 0;
  const skillsTotal = caps?.skills.length ?? 0;
  const mcpConnected = caps?.servers.filter((srv) => srv.status === "connected").length ?? 0;
  const mcpTotal = caps?.servers.length ?? 0;
  const balanceLabel = balance?.available && balance.display ? balance.display : "-";

  return (
    <>
      <section className="settings-block">
        <div className="usage-chart-card__head">
          <h3 className="settings-block__title usage-chart-card__title">{t("settings.usage.chartTitle")}</h3>
          <Tooltip label={t("settings.usage.refresh")}>
            <button
              type="button"
              className="usage-chart-card__refresh"
              onClick={() => void refresh()}
              disabled={loading}
              aria-label={t("settings.usage.refresh")}
            >
              {loading ? <Loader2 size={14} className="dock-panel__spin" /> : <RefreshCw size={14} />}
            </button>
          </Tooltip>
        </div>
        <div className="settings-block__card">
          <div className="settings-block__card-content">
            <UsageActivityChart points={chartPoints} sample={usingSample} currency={currency} />
          </div>
        </div>
      </section>

      <section className="settings-block">
        <h3 className="settings-block__title">{t("settings.usage.overviewTitle")}</h3>
        <div className="settings-block__card">
          <p className="settings-block__card-lead">{t("settings.usage.overviewHint")}</p>
          <div className="settings-block__card-content">
            <div className="usage-stat-grid">
              <StatCard label={t("settings.usage.totalTokens")} value={formatTokens(summary.totalTokens)} hint={t("settings.usage.totalTokensHint")} />
              <StatCard
                label={t("settings.usage.balance")}
                value={balanceLabel}
                hint={balance?.available ? t("settings.usage.balanceHint") : t("settings.usage.balanceUnavailable")}
              />
              <StatCard
                label={t("settings.usage.skills")}
                value={String(skillsEnabled)}
                hint={t("settings.usage.skillsHint", { total: String(skillsTotal) })}
              />
              <StatCard label={t("settings.usage.todayTokens")} value={formatTokens(summary.todayTokens)} />
              <StatCard label={t("settings.usage.todayCost")} value={formatMoney(summary.todayCost, currency)} hint={t("settings.usage.todayCostHint")} />
              <StatCard label={t("settings.usage.totalCost")} value={formatMoney(summary.totalCost, currency)} hint={t("settings.usage.totalCostHint")} />
              <StatCard
                label={t("settings.usage.cacheHit")}
                value={summary.avgCacheHitPct !== null ? `${summary.avgCacheHitPct}%` : "-"}
                hint={t("settings.usage.cacheHitHint")}
              />
              <StatCard label={t("settings.usage.totalTurns")} value={String(summary.totalTurns)} hint={t("settings.usage.totalTurnsHint")} />
              <StatCard
                label={t("settings.usage.contextCompactions")}
                value={String(summary.totalContextCompactions)}
                hint={t("settings.usage.contextCompactionsHint")}
              />
              <StatCard
                label={t("settings.usage.sessionCompactions")}
                value={String(summary.totalSessionCompactions)}
                hint={t("settings.usage.sessionCompactionsHint")}
              />
              <StatCard label={t("settings.usage.activeDays")} value={String(summary.activeDays)} hint={t("settings.usage.activeDaysHint")} />
              <StatCard
                label={t("settings.usage.mcpServers")}
                value={String(mcpConnected)}
                hint={t("settings.usage.mcpServersHint", { total: String(mcpTotal) })}
              />
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
