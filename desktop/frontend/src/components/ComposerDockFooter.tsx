import { useT } from "../lib/i18n";
import type { BalanceInfo, ContextInfo, WireUsage } from "../lib/types";

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

function currencySymbol(currency?: string): string {
  const value = (currency || "¥").trim();
  if (/^(cny|rmb|yuan)$/i.test(value)) return "¥";
  if (/^(usd|dollar)$/i.test(value)) return "$";
  return value || "¥";
}

function formatMoney(amount?: number, currency?: string): string {
  const symbol = currencySymbol(currency);
  if (typeof amount !== "number" || amount <= 0) return `${symbol}0.0000`;
  return `${symbol}${amount < 1 ? amount.toFixed(4) : amount.toFixed(2)}`;
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
  const hitPct = nowRate(usage);
  const avgPct = avgRate(usage);
  const costLabel = formatMoney(sessionCost, sessionCurrency);
  const balanceLabel = balance?.available && balance.display ? balance.display : "-";

  const parts: string[] = [
    pct !== null ? t("status.ctx", { pct }) : t("status.ctxUnknown"),
    compactPct !== null ? t("status.compact", { pct: compactPct }) : t("status.compactUnknown"),
    t("status.cache", { pct: hitPct ?? "-" }),
    t("status.cacheAvg", { pct: avgPct ?? "-" }),
  ];

  if (terminalCount > 0) {
    parts.push(t("composer.footer.terminals", { n: terminalCount }));
  }

  parts.push(t("status.cost", { amount: costLabel }));
  parts.push(t("status.balance", { amount: balanceLabel }));

  return (
    <div className="composer-dock__footer" aria-label={t("composer.footer.aria")}>
      {parts.map((part, index) => (
        <span key={part} className="composer-dock__footer-part">
          {index > 0 ? <span className="composer-dock__footer-sep">·</span> : null}
          {part}
        </span>
      ))}
    </div>
  );
}
