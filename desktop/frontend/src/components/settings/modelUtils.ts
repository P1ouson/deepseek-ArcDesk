import { asArray } from "../../lib/array";
import type { ProviderView, SettingsView } from "../../lib/types";

export const DEEPSEEK_OFFICIAL_BASE = "https://api.deepseek.com";

export function deepseekProviders(providers: ProviderView[]): ProviderView[] {
  return asArray(providers).filter((p) => p.apiKeyEnv === "DEEPSEEK_API_KEY");
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
