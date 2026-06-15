import type { ReactNode } from "react";
import { useT } from "../lib/i18n";
import { formatMoney } from "../lib/formatMoney";
import { sessionCacheRate, stepCacheRate } from "../lib/cacheRate";
import { Tooltip } from "./Tooltip";
import type { BalanceInfo, ContextInfo, WireUsage } from "../lib/types";

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
  const stepPct = stepCacheRate(usage);
  const avgPct = sessionCacheRate(usage);
  const billingAvailable = balance?.available === true;
  const costLabel = billingAvailable ? formatMoney(sessionCost, sessionCurrency) : "--";
  const balanceLabel = billingAvailable && balance.display ? balance.display : "--";

  // Reasonix desktop order: 本次命中 (latest step) primary · 平均命中 secondary.
  const cachePrimary = (
    <Tooltip label={t("status.cacheTitle")} side="top">
      <span className="composer-dock__footer-cache-primary">
        {t("status.cacheLabel")} {stepPct ?? "-"}%
      </span>
    </Tooltip>
  );

  const cacheSecondary = (
    <Tooltip label={t("status.cacheAvgTitle")} side="top">
      <span className="composer-dock__footer-cache-secondary">
        {t("status.cacheAvgLabel")} {avgPct ?? "-"}%
      </span>
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
