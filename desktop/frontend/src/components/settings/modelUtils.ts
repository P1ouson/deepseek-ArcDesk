import { asArray } from "../../lib/array";
import type { Translator } from "../../lib/i18n";
import { modelLabelFromRef, modelShortLabel } from "../../lib/modelLabel";
import type { ProviderView, SettingsView } from "../../lib/types";

export { modelLabelFromRef, modelShortLabel };
export const DEEPSEEK_OFFICIAL_BASE = "https://api.deepseek.com";

const OPENAI_BASE_SUFFIXES = ["/chat/completions", "/completions", "/embeddings", "/models"];

/** Strip endpoint paths users paste by mistake; keep up to /v1. */
export function normalizeProviderBaseUrl(raw: string): string {
  let base = raw.trim().replace(/\/+$/, "");
  if (!base) return DEEPSEEK_OFFICIAL_BASE;
  for (;;) {
    const lower = base.toLowerCase();
    const hit = OPENAI_BASE_SUFFIXES.find((suffix) => lower.endsWith(suffix));
    if (!hit) break;
    base = base.slice(0, -hit.length).replace(/\/+$/, "");
  }
  return base;
}

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

export function formatModelFetchError(raw: string, relayMode: boolean, t: Translator): string {
  const msg = raw.replace(/^fetch models:\s*/i, "").replace(/^validate:\s*/i, "").trim();
  if (
    relayMode &&
    /status\s*401|status\s*403|无效的令牌|未提供令牌|unauthorized|invalid.*token/i.test(msg)
  ) {
    return t("settings.models.relayAuthError");
  }
  if (
    relayMode &&
    (/status\s*404|invalid url/i.test(msg) && /chat\/completions\/models|\/completions\/models/i.test(msg))
  ) {
    return t("settings.models.relayBaseUrlError");
  }
  if (
    relayMode &&
    /eof|connection reset|broken pipe|tls:|timeout|dial tcp|i\/o timeout|network is unreachable/i.test(msg)
  ) {
    return t("settings.models.relayNetworkError");
  }
  return msg.replace(/^request failed:\s*/i, "").trim() || raw;
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
