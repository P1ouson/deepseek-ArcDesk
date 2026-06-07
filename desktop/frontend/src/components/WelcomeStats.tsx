import { useEffect, useMemo, useState, type CSSProperties } from "react";
import logoWordmark from "../assets/logo-wordmark.svg";
import { app } from "../lib/bridge";
import { formatMoney, formatTokens } from "../lib/formatMoney";
import { useT } from "../lib/i18n";
import type { ContextPanelInfo, WireUsage } from "../lib/types";

interface WelcomeStatsProps {
  tabId?: string;
  usage?: WireUsage;
  sessionCost?: number;
  sessionCurrency?: string;
  onPrompt: (text: string) => void;
}

function tokenRingStyle(usage?: WireUsage): CSSProperties {
  const prompt = usage?.promptTokens ?? 0;
  const completion = usage?.completionTokens ?? 0;
  const reasoning = usage?.reasoningTokens ?? 0;
  const total = Math.max(prompt + completion + reasoning, 1);
  const p1 = (prompt / total) * 100;
  const p2 = p1 + (completion / total) * 100;
  return {
    background: `conic-gradient(var(--accent) 0 ${p1}%, var(--success) ${p1}% ${p2}%, var(--skill) ${p2}% 100%)`,
  };
}

export function WelcomeStats({ tabId, usage, sessionCost, sessionCurrency, onPrompt }: WelcomeStatsProps) {
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
  const savings = usage?.sessionCacheHitTokens ? usage.sessionCacheHitTokens * 0.000002 : 0.1272;

  const quickTasks = useMemo(
    () => [
      t("welcome.ex1"),
      t("welcome.ex2"),
      t("welcome.ex3"),
    ],
    [t],
  );

  return (
    <div className="welcome-stats">
      <div className="welcome-stats__brand">
        <img src={logoWordmark} alt="Reasonix" className="welcome-stats__logo" />
        <div className="welcome-stats__tagline">{t("welcome.tagline")}</div>
      </div>

      <div className="welcome-stats__card">
        <div className="welcome-stats__metrics">
          <div className="welcome-stats__ring" style={tokenRingStyle(usage)}>
            <div className="welcome-stats__ring-inner">{formatTokens(totalTokens)}</div>
          </div>
          <div className="welcome-stats__stat-rows">
            <div className="welcome-stats__stat-row">
              <span>{t("context.total")}</span>
              <span>{formatTokens(totalTokens)}</span>
            </div>
            <div className="welcome-stats__stat-row">
              <span>{t("welcome.cacheHit")}</span>
              <span>{cachePct ? `${cachePct}%` : "-"}</span>
            </div>
            <div className="welcome-stats__stat-row">
              <span>{t("status.spendTitle")}</span>
              <span>{formatMoney(cost, currency)}</span>
            </div>
          </div>
        </div>

        <div className="welcome-stats__grid">
          <div className="welcome-stats__grid-cell">
            <div className="welcome-stats__grid-label">{t("sidebar.conversations")}</div>
            <div className="welcome-stats__grid-value">1</div>
          </div>
          <div className="welcome-stats__grid-cell">
            <div className="welcome-stats__grid-label">{t("welcome.messages")}</div>
            <div className="welcome-stats__grid-value">{usage?.totalTokens ? Math.max(1, Math.round(usage.totalTokens / 1000)) : 0}</div>
          </div>
          <div className="welcome-stats__grid-cell">
            <div className="welcome-stats__grid-label">{t("welcome.activeDays")}</div>
            <div className="welcome-stats__grid-value">1</div>
          </div>
          <div className="welcome-stats__grid-cell">
            <div className="welcome-stats__grid-label">{t("context.total")}</div>
            <div className="welcome-stats__grid-value">{formatMoney(cost, currency)}</div>
          </div>
        </div>

        {savings > 0 && (
          <div className="welcome-stats__savings">{t("welcome.savings", { amount: formatMoney(savings, currency) })}</div>
        )}
      </div>

      <div className="welcome-stats__tasks">
        {quickTasks.map((task) => (
          <button key={task} type="button" className="welcome-stats__task" onClick={() => onPrompt(task)}>
            {task}
          </button>
        ))}
      </div>
    </div>
  );
}
