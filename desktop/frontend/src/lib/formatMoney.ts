import { getLocale } from "./i18n";

/** Normalize legacy/typo currency codes (e.g. config typo 楼 → CNY). */
export function normalizeCurrency(currency?: string): string {
  const value = (currency || "CNY").trim();
  if (/^楼$/u.test(value)) return "CNY";
  return value;
}

export function currencySymbol(currency?: string): string {
  const value = normalizeCurrency(currency);
  if (/^(cny|rmb|yuan|¥)$/i.test(value)) return getLocale() === "zh" ? "" : "¥";
  if (/^(usd|dollar|\$)$/i.test(value)) return "$";
  if (value.length === 1) return value;
  return getLocale() === "zh" ? "" : "¥";
}

function moneySuffix(currency?: string): string {
  const value = normalizeCurrency(currency);
  if (/^(cny|rmb|yuan|¥)$/i.test(value) && getLocale() === "zh") return "元";
  return "";
}

export function formatMoney(amount?: number, currency?: string): string {
  const symbol = currencySymbol(currency);
  const suffix = moneySuffix(currency);
  if (typeof amount !== "number" || amount <= 0) {
    return suffix ? `0.0000${suffix}` : `${symbol}0.0000`;
  }
  const num = amount < 1 ? amount.toFixed(4) : amount.toFixed(2);
  return suffix ? `${num}${suffix}` : `${symbol}${num}`;
}

export function formatTokens(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}k`;
  return String(n);
}
