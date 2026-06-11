import type { ReactNode } from "react";
import { useT, type Translator } from "../lib/i18n";
import { formatMoney } from "../lib/formatMoney";
import { Tooltip } from "./Tooltip";
import type { BalanceInfo, ContextInfo, WireCacheDiagnostics, WireUsage } from "../lib/types";

function formatRate(hit: number, denom: number): string | null {
  if (denom <= 0) return null;
  return ((hit / denom) * 100).toFixed(1);
}

function stepRate(u?: WireUsage): string | null {
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

function prefixWarning(
  diag: WireCacheDiagnostics | undefined,
  t: Translator,
): string | null {
  if (!diag?.prefixChanged) return null;
  const reasons = diag.prefixChangeReasons?.filter(Boolean) ?? [];
  if (reasons.length === 0) return t("status.cachePrefixWarnUnknown");
  return t("status.cachePrefixWarn", { reasons: reasons.join(", ") });
}

export interface ComposerDockFooterProps {
  context: ContextInfo;
  usage?: WireUsage;
  balance?: BalanceInfo;
  sessionCost?: number;
  sessionCurrency?: string;
  terminalCount?: number;
}

export function ComposerDockFooter({
  context,
  usage,
  balance,
  sessionCost,
  sessionCurrency,
  terminalCount = 0,
}: ComposerDockFooterProps) {
  const t = useT();
  const pct = context.window ? Math.min(100, Math.round((context.used / context.window) * 100)) : null;
  const compactPct = context.compactRatio ? Math.round(context.compactRatio * 100) : null;
  const avgPct = avgRate(usage);
  const stepPct = stepRate(usage);
  const costLabel = formatMoney(sessionCost, sessionCurrency);
  const balanceLabel = balance?.available && balance.display ? balance.display : "-";
  const prefixWarn = prefixWarning(usage?.cacheDiagnostics, t);

  const cachePrimary = (
    <Tooltip label={t("status.cacheAvgTip")} side="top">
      <span className="composer-dock__footer-cache-primary">{t("status.cacheAvg", { pct: avgPct ?? "-" })}</span>
    </Tooltip>
  );

  const cacheSecondary = (
    <Tooltip label={t("status.cacheStepTip")} side="top">
      <span className="composer-dock__footer-cache-secondary">{t("status.cacheStep", { pct: stepPct ?? "-" })}</span>
    </Tooltip>
  );

  const parts: Array<string | ReactNode> = [
    pct !== null ? t("status.ctx", { pct }) : t("status.ctxUnknown"),
    compactPct !== null ? t("status.compact", { pct: compactPct }) : t("status.compactUnknown"),
    <span key="cache" className="composer-dock__footer-cache">
      {cachePrimary}
      <span className="composer-dock__footer-sep">·</span>
      {cacheSecondary}
    </span>,
  ];

  if (terminalCount > 0) {
    parts.push(t("composer.footer.terminals", { n: terminalCount }));
  }

  parts.push(t("status.cost", { amount: costLabel }));
  parts.push(t("status.balance", { amount: balanceLabel }));

  return (
    <div className="composer-dock__footer" aria-label={t("composer.footer.aria")}>
      {prefixWarn ? <div className="composer-dock__footer-prefix-warn">{prefixWarn}</div> : null}
      <div className="composer-dock__footer-row">
        {parts.map((part, index) => (
          <span key={index} className="composer-dock__footer-part">
            {index > 0 ? <span className="composer-dock__footer-sep">·</span> : null}
            {part}
          </span>
        ))}
      </div>
    </div>
  );
}
