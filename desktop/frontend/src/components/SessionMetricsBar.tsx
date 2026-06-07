import { useEffect, useState } from "react";
import { app } from "../lib/bridge";
import { formatMoney, formatTokens } from "../lib/formatMoney";
import { useT } from "../lib/i18n";
import type { BalanceInfo, ContextPanelInfo, WireUsage } from "../lib/types";

export function SessionMetricsBar({
  tabId,
  usage,
  sessionCost,
  sessionCurrency,
  balance,
}: {
  tabId?: string;
  usage?: WireUsage;
  sessionCost?: number;
  sessionCurrency?: string;
  balance?: BalanceInfo;
}) {
  const t = useT();
  const [panel, setPanel] = useState<ContextPanelInfo | null>(null);

  useEffect(() => {
    if (!tabId) return;
    void app.ContextPanel(tabId)
      .then(setPanel)
      .catch(() => setPanel(null));
  }, [tabId]);

  const totalTokens = panel?.usedTokens ?? usage?.totalTokens ?? 0;
  const cacheDenom = (usage?.cacheHitTokens ?? 0) + (usage?.cacheMissTokens ?? 0);
  const cachePct = cacheDenom > 0 ? Math.round(((usage?.cacheHitTokens ?? 0) / cacheDenom) * 100) : 0;
  const cost = sessionCost ?? panel?.sessionCost ?? 0;
  const currency = sessionCurrency ?? panel?.sessionCurrency ?? "CNY";
  const balanceText = balance?.available && balance.display ? balance.display : "-";

  return (
    <div className="session-metrics" role="status" aria-label={t("welcome.metricsAria")}>
      <div className="session-metrics__item">
        <span className="session-metrics__label">{t("status.tokens")}</span>
        <span className="session-metrics__value">{formatTokens(totalTokens)}</span>
      </div>
      <div className="session-metrics__item">
        <span className="session-metrics__label">{t("status.spendTitle")}</span>
        <span className="session-metrics__value session-metrics__value--money">{formatMoney(cost, currency)}</span>
      </div>
      <div className="session-metrics__item">
        <span className="session-metrics__label">{t("status.balanceTitle")}</span>
        <span className="session-metrics__value session-metrics__value--money">{balanceText}</span>
      </div>
      {cachePct > 0 && (
        <div className="session-metrics__item">
          <span className="session-metrics__label">{t("welcome.cacheHit")}</span>
          <span className="session-metrics__value">{cachePct}%</span>
        </div>
      )}
    </div>
  );
}
