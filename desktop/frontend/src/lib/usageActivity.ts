import type { WireUsage } from "./types";

export interface UsageDayBucket {
  date: string;
  tokens: number;
  cost: number;
  turns: number;
  cacheHitTokens: number;
  cacheMissTokens: number;
  contextCompactions: number;
  sessionCompactions: number;
}

export interface UsageChartPoint {
  date: string;
  label: string;
  tokens: number;
  cost: number;
  turns: number;
  sample?: boolean;
}

export interface UsageActivitySummary {
  totalTokens: number;
  totalCost: number;
  activeDays: number;
  todayTokens: number;
  todayCost: number;
  totalTurns: number;
  avgCacheHitPct: number | null;
  totalContextCompactions: number;
  totalSessionCompactions: number;
}

const STORAGE_KEY = "arcdesk.usageActivity.v1";
const LEGACY_STORAGE_KEY = "reasonix.usageActivity.v1";
const MAX_DAYS = 90;
const UPDATE_EVENT = "usage-activity-updated";

function migrateLegacyUsageStorage(): void {
  try {
    const legacy = localStorage.getItem(LEGACY_STORAGE_KEY);
    if (!legacy || localStorage.getItem(STORAGE_KEY)) return;
    localStorage.setItem(STORAGE_KEY, legacy);
    localStorage.removeItem(LEGACY_STORAGE_KEY);
  } catch {
    /* storage unavailable */
  }
}

migrateLegacyUsageStorage();

function todayKey(): string {
  const now = new Date();
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  return `${y}-${m}-${d}`;
}

function normalizeBucket(row: Partial<UsageDayBucket> & { date: string }): UsageDayBucket {
  return {
    date: row.date,
    tokens: Math.max(0, Number(row.tokens) || 0),
    cost: Math.max(0, Number(row.cost) || 0),
    turns: Math.max(0, Number(row.turns) || 0),
    cacheHitTokens: Math.max(0, Number(row.cacheHitTokens) || 0),
    cacheMissTokens: Math.max(0, Number(row.cacheMissTokens) || 0),
    contextCompactions: Math.max(0, Number(row.contextCompactions) || 0),
    sessionCompactions: Math.max(0, Number(row.sessionCompactions) || 0),
  };
}

function eventTokens(usage: WireUsage): number {
  const prompt = usage.promptTokens ?? 0;
  const completion = usage.completionTokens ?? 0;
  const reasoning = usage.reasoningTokens ?? 0;
  const total = prompt + completion + reasoning;
  if (total > 0) return total;
  return usage.totalTokens ?? 0;
}

function eventCost(usage: WireUsage): number {
  return usage.cost ?? usage.costUsd ?? 0;
}

function readBuckets(): UsageDayBucket[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as UsageDayBucket[];
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter((row) => row && typeof row.date === "string")
      .map((row) => normalizeBucket(row))
      .sort((a, b) => a.date.localeCompare(b.date))
      .slice(-MAX_DAYS);
  } catch {
    return [];
  }
}

function writeBuckets(buckets: UsageDayBucket[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(buckets.slice(-MAX_DAYS)));
    window.dispatchEvent(new CustomEvent(UPDATE_EVENT));
  } catch {
    /* storage unavailable */
  }
}

function upsertToday(mutate: (row: UsageDayBucket) => UsageDayBucket): void {
  const key = todayKey();
  const buckets = readBuckets();
  const idx = buckets.findIndex((row) => row.date === key);
  if (idx >= 0) {
    buckets[idx] = mutate(buckets[idx]);
  } else {
    buckets.push(
      mutate({
        date: key,
        tokens: 0,
        cost: 0,
        turns: 0,
        cacheHitTokens: 0,
        cacheMissTokens: 0,
        contextCompactions: 0,
        sessionCompactions: 0,
      }),
    );
  }
  writeBuckets(buckets);
}

export function recordUsageActivity(usage?: WireUsage): void {
  if (!usage) return;
  const tokens = eventTokens(usage);
  const cost = eventCost(usage);
  const cacheHit = usage.cacheHitTokens ?? 0;
  const cacheMiss = usage.cacheMissTokens ?? 0;
  if (tokens <= 0 && cost <= 0 && cacheHit <= 0 && cacheMiss <= 0) return;

  upsertToday((row) => ({
    ...row,
    tokens: row.tokens + tokens,
    cost: row.cost + cost,
    turns: row.turns + 1,
    cacheHitTokens: row.cacheHitTokens + cacheHit,
    cacheMissTokens: row.cacheMissTokens + cacheMiss,
  }));
}

export function recordCompactionActivity(trigger?: string): void {
  const manual = (trigger ?? "").toLowerCase() === "manual";
  upsertToday((row) => ({
    ...row,
    contextCompactions: row.contextCompactions + (manual ? 0 : 1),
    sessionCompactions: row.sessionCompactions + (manual ? 1 : 0),
  }));
}

export function loadUsageActivity(): UsageDayBucket[] {
  return readBuckets();
}

export function hasUsageActivity(buckets: UsageDayBucket[]): boolean {
  return buckets.some((row) => row.tokens > 0 || row.cost > 0 || row.turns > 0);
}

export function summarizeUsageActivity(buckets: UsageDayBucket[]): UsageActivitySummary {
  const today = todayKey();
  let totalTokens = 0;
  let totalCost = 0;
  let activeDays = 0;
  let todayTokens = 0;
  let todayCost = 0;
  let totalTurns = 0;
  let cacheHit = 0;
  let cacheMiss = 0;
  let totalContextCompactions = 0;
  let totalSessionCompactions = 0;

  for (const row of buckets) {
    totalTokens += row.tokens;
    totalCost += row.cost;
    totalTurns += row.turns;
    cacheHit += row.cacheHitTokens;
    cacheMiss += row.cacheMissTokens;
    totalContextCompactions += row.contextCompactions;
    totalSessionCompactions += row.sessionCompactions;
    if (row.tokens > 0 || row.cost > 0 || row.turns > 0) activeDays += 1;
    if (row.date === today) {
      todayTokens = row.tokens;
      todayCost = row.cost;
    }
  }

  const cacheDenom = cacheHit + cacheMiss;
  const avgCacheHitPct = cacheDenom > 0 ? Math.round((cacheHit / cacheDenom) * 100) : null;

  return {
    totalTokens,
    totalCost,
    activeDays,
    todayTokens,
    todayCost,
    totalTurns,
    avgCacheHitPct,
    totalContextCompactions,
    totalSessionCompactions,
  };
}

function dateLabel(day: Date): { date: string; label: string } {
  const y = day.getFullYear();
  const m = String(day.getMonth() + 1).padStart(2, "0");
  const d = String(day.getDate()).padStart(2, "0");
  return { date: `${y}-${m}-${d}`, label: `${m}/${d}` };
}

export function buildUsageChart(buckets: UsageDayBucket[], days = 14): UsageChartPoint[] {
  const map = new Map(buckets.map((row) => [row.date, row]));
  const points: UsageChartPoint[] = [];
  const cursor = new Date();
  cursor.setHours(12, 0, 0, 0);

  for (let i = days - 1; i >= 0; i--) {
    const day = new Date(cursor);
    day.setDate(cursor.getDate() - i);
    const { date, label } = dateLabel(day);
    const row = map.get(date);
    points.push({
      date,
      label,
      tokens: row?.tokens ?? 0,
      cost: row?.cost ?? 0,
      turns: row?.turns ?? 0,
    });
  }

  return points;
}

/** Pixel height for usage chart bars (matches `.usage-chart__bars` area). */
export const USAGE_CHART_BAR_MAX_PX = 148;

export function usageChartBarHeightPx(tokens: number, maxTokens: number): number {
  if (tokens <= 0) return 4;
  const max = Math.max(maxTokens, 1);
  return Math.max(8, Math.round((tokens / max) * USAGE_CHART_BAR_MAX_PX));
}

export const USAGE_ACTIVITY_UPDATE_EVENT = UPDATE_EVENT;
