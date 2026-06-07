import { useState } from "react";
import { Tooltip } from "./Tooltip";
import { useI18n } from "../lib/i18n";
import { formatMoney } from "../lib/formatMoney";
import type { BalanceInfo, ContextInfo, JobView, Mode, WireUsage } from "../lib/types";

function JobsChip({ jobs }: { jobs: JobView[] }) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  if (jobs.length === 0) {
    return <span className="status-strip__item">{t("status.jobsCount", { n: 0 })}</span>;
  }
  return (
    <div className="status-strip__jobs-wrap">
      <Tooltip label={t("status.jobsTitle")}>
        <button type="button" className="status-strip__jobs" onClick={() => setOpen((v) => !v)}>
          {t("status.jobsCount", { n: jobs.length })}
        </button>
      </Tooltip>
      {open && (
        <>
          <div className="modelsw__backdrop" onClick={() => setOpen(false)} />
          <div className="modelsw__menu jobsmenu" role="listbox">
            <div className="jobsmenu__head">{t("status.jobsTitle")}</div>
            {jobs.map((j) => (
              <div className="jobsmenu__item" key={j.id} role="option">
                <span className="jobsmenu__id">{j.id}</span>
                <span className="jobsmenu__label">{j.label || j.kind}</span>
                <span className="jobsmenu__status">{j.status}</span>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

function formatRate(hit: number, denom: number): string | null {
  if (denom <= 0) return null;
  return ((hit / denom) * 100).toFixed(1);
}

function nowRate(u?: WireUsage): string | null {
  if (!u) return null;
  let denom = u.cacheHitTokens + u.cacheMissTokens;
  if (denom === 0) denom = u.promptTokens;
  return formatRate(u.cacheHitTokens, denom);
}

function avgRate(u?: WireUsage): string | null {
  if (!u) return null;
  const denom = u.sessionCacheHitTokens + u.sessionCacheMissTokens;
  return formatRate(u.sessionCacheHitTokens, denom);
}

function modeDotClass(mode: Mode, running: boolean): string {
  if (running) return "status-strip__dot status-strip__dot--busy";
  if (mode === "plan") return "status-strip__dot status-strip__dot--plan";
  return "status-strip__dot";
}

export function StatusStrip({
  context,
  usage,
  balance,
  jobs,
  running,
  mode,
  cost,
  currency,
  connected = true,
}: {
  context: ContextInfo;
  usage?: WireUsage;
  balance?: BalanceInfo;
  jobs?: JobView[];
  running: boolean;
  mode: Mode;
  cost?: number;
  currency?: string;
  connected?: boolean;
}) {
  const { t } = useI18n();
  const used = context.used ?? 0;
  const windowSize = context.window ?? 0;
  const pct = windowSize ? Math.min(100, Math.round((used / windowSize) * 100)) : null;
  const compactPct = context.compactRatio ? Math.round(context.compactRatio * 100) : null;
  const nowPct = nowRate(usage);
  const avgPct = avgRate(usage);
  const modeLabel = mode === "plan" ? t("status.plan") : mode === "yolo" ? "agent" : "normal";

  return (
    <div className="status-strip">
      <span className={modeDotClass(mode, running)} aria-hidden />
      <span className="status-strip__item">{modeLabel}</span>
      <span className="status-strip__sep">·</span>
      <span className="status-strip__item">
        {pct !== null ? t("status.ctx", { pct }) : t("status.ctxUnknown")}
      </span>
      <span className="status-strip__sep">·</span>
      <span className="status-strip__item">
        {compactPct !== null ? t("status.compact", { pct: compactPct }) : t("status.compactUnknown")}
      </span>
      <span className="status-strip__sep">·</span>
      <span className="status-strip__item">{t("status.cache", { pct: nowPct ?? "-" })}</span>
      <span className="status-strip__sep">·</span>
      <span className="status-strip__item">{t("status.cacheAvg", { pct: avgPct ?? "-" })}</span>
      <span className="status-strip__spacer" />
      <JobsChip jobs={jobs ?? []} />
      <span className="status-strip__sep">·</span>
      <Tooltip label={t("status.spendTitle")}>
        <span className="status-strip__item">{formatMoney(cost, currency)}</span>
      </Tooltip>
      <span className="status-strip__sep">·</span>
      <Tooltip label={t("status.balanceTitle")}>
        <span className="status-strip__item">{balance?.display ?? "-"}</span>
      </Tooltip>
      <span className="status-strip__sep">·</span>
      <span className="status-strip__item">{connected ? t("status.connected") : t("status.offline")}</span>
    </div>
  );
}
