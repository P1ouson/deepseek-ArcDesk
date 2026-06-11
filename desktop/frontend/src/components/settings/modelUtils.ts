import { asArray } from "../../lib/array";
import { modelLabelFromRef, modelShortLabel } from "../../lib/modelLabel";
import type { ProviderView, SettingsView } from "../../lib/types";

export { modelLabelFromRef, modelShortLabel };
export const DEEPSEEK_OFFICIAL_BASE = "https://api.deepseek.com";

export function isRelayBaseUrl(baseUrl: string | undefined | null): boolean {
  const base = (baseUrl ?? "").trim().replace(/\/$/, "");
  if (!base) return false;
  try {
    const host = new URL(base).hostname.toLowerCase();
    return host !== "api.deepseek.com";
  } catch {
    return true;
  }
}

export function deepseekProviders(providers: ProviderView[]): ProviderView[] {
  return asArray(providers).filter((p) => p.apiKeyEnv === "DEEPSEEK_API_KEY");
}

/** Union of synced GET /models ids across DeepSeek-key providers (API list only). */
export function deepseekSyncedModels(providers: ProviderView[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const p of deepseekProviders(providers)) {
    for (const m of p.models ?? []) {
      const id = m.trim();
      if (!id || seen.has(id)) continue;
      seen.add(id);
      out.push(id);
    }
  }
  return out;
}

export function primaryApiProvider(providers: ProviderView[]): ProviderView | undefined {
  const list = deepseekProviders(providers);
  if (list.length === 0) return providers[0];
  return list.reduce((best, p) => {
    const count = p.models?.length ?? 0;
    const bestCount = best.models?.length ?? 0;
    return count > bestCount ? p : best;
  }, list[0]);
}

export function modelRef(providerName: string, modelId: string): string {
  return `${providerName}/${modelId}`;
}

export function looksLikeStalePresetModels(models: string[]): boolean {
  return (
    models.length > 0 &&
    models.length <= 2 &&
    models.every((m) => /^deepseek-/i.test(m))
  );
}

// toRef normalises a stored model id (a provider name, a bare model, or a ref) to
// a "provider/model" ref so a <select> of refs can show it selected.
export function toRef(model: string, s: SettingsView): string {
  if (!model) return "";
  if (model.includes("/")) return model;
  const byName = s.providers.find((p) => p.name === model);
  if (byName) return `${byName.name}/${byName.default || byName.models[0] || ""}`;
  const byModel = s.providers.find((p) => p.models.includes(model));
  if (byModel) return `${byModel.name}/${model}`;
  return model;
}

export function providerNames(providers: ProviderView[]): { value: string; label: string }[] {
  return [{ value: "", label: "—" }, ...providers.map((p) => ({ value: p.name, label: p.name }))];
}
