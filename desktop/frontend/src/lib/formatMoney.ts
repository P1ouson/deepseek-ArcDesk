export function currencySymbol(currency?: string): string {
  const value = (currency || "CNY").trim();
  if (/^(cny|rmb|yuan|¥)$/i.test(value)) return "¥";
  if (/^(usd|dollar|\$)$/i.test(value)) return "$";
  if (value.length === 1) return value;
  return "¥";
}

export function formatMoney(amount?: number, currency?: string): string {
  const symbol = currencySymbol(currency);
  if (typeof amount !== "number" || amount <= 0) return `${symbol}0.0000`;
  return `${symbol}${amount < 1 ? amount.toFixed(4) : amount.toFixed(2)}`;
}

export function formatTokens(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}k`;
  return String(n);
}
