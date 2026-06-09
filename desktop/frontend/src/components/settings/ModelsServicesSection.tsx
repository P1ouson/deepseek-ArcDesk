import { useEffect, useRef, useState } from "react";
import { ChevronDown, Loader2 } from "lucide-react";
import { app } from "../../lib/bridge";
import { useT } from "../../lib/i18n";
import { useDismissOnOutsidePointerDown } from "../../lib/useDismissOnOutsidePointerDown";
import type { ProviderView } from "../../lib/types";
import { confirmAction } from "../../lib/confirmAction";
import { StudioSelect } from "../StudioSelect";
import {
  SettingsBlock,
  SettingsSaveChip,
  type SettingsSectionProps,
} from "../settingsPrimitives";
import { DEEPSEEK_OFFICIAL_BASE, deepseekProviders, toRef } from "./modelUtils";

function DeepSeekServiceUrlField({
  providers,
  busy,
  apply,
}: {
  providers: ProviderView[];
  busy: boolean;
  apply: SettingsSectionProps["apply"];
}) {
  const t = useT();
  const targets = deepseekProviders(providers);
  const currentBase = targets[0]?.baseUrl ?? DEEPSEEK_OFFICIAL_BASE;
  const [value, setValue] = useState(currentBase === DEEPSEEK_OFFICIAL_BASE ? "" : currentBase);

  useEffect(() => {
    const next = targets[0]?.baseUrl ?? DEEPSEEK_OFFICIAL_BASE;
    setValue(next === DEEPSEEK_OFFICIAL_BASE ? "" : next);
  }, [targets[0]?.baseUrl]);

  const savedDisplay =
    (targets[0]?.baseUrl ?? DEEPSEEK_OFFICIAL_BASE) === DEEPSEEK_OFFICIAL_BASE
      ? ""
      : (targets[0]?.baseUrl ?? "");
  const dirty = value !== savedDisplay;

  const save = () => {
    const next = value.trim() || DEEPSEEK_OFFICIAL_BASE;
    void apply(async () => {
      for (const provider of targets) {
        await app.SaveProvider({ ...provider, baseUrl: next });
      }
    });
  };

  return (
    <div className="settings-block__inline-save">
      <input
        className="mem-input settings-block__input"
        value={value}
        disabled={busy || !targets.length}
        placeholder={t("settings.models.baseUrlPlaceholder")}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && dirty) save();
        }}
      />
      <SettingsSaveChip disabled={busy || !targets.length || !dirty} ready={dirty} onClick={save}>
        {t("common.save")}
      </SettingsSaveChip>
    </div>
  );
}

function KeyField({ apiKeyEnv, busy, onSet }: { apiKeyEnv: string; busy: boolean; onSet: (v: string) => Promise<void> }) {
  const t = useT();
  const [val, setVal] = useState("");
  if (!apiKeyEnv) return null;
  return (
    <div className="set-key">
      <input
        className="mem-input"
        type="password"
        placeholder={t("settings.setKey", { env: apiKeyEnv })}
        value={val}
        onChange={(e) => setVal(e.target.value)}
      />
      <SettingsSaveChip
        disabled={busy || !val.trim()}
        ready={!!val.trim()}
        onClick={() => {
          void onSet(val.trim());
          setVal("");
        }}
      >
        {t("settings.saveKey")}
      </SettingsSaveChip>
    </div>
  );
}

export function ModelsServicesSection({ s, busy, apply }: SettingsSectionProps) {
  const t = useT();
  const deepseek = deepseekProviders(s.providers)[0] ?? s.providers[0];
  const [fetchedModels, setFetchedModels] = useState<string[] | null>(null);
  const [modelsExpanded, setModelsExpanded] = useState(false);
  const [fetchCount, setFetchCount] = useState<number | null>(null);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [fetching, setFetching] = useState(false);
  const modelsToggleRef = useRef<HTMLButtonElement>(null);
  const modelsPanelRef = useRef<HTMLDivElement>(null);

  useDismissOnOutsidePointerDown(modelsExpanded, () => setModelsExpanded(false), {
    excludeRefs: [modelsToggleRef, modelsPanelRef],
  });

  const modelRefs =
    fetchedModels && deepseek ? fetchedModels.map((model) => `${deepseek.name}/${model}`) : [];
  const modelsReady = modelRefs.length > 0;
  const selectPlaceholder = t("settings.models.selectAfterFetch");

  const syncModels = () => {
    if (!deepseek) return;
    setFetching(true);
    setFetchCount(null);
    setFetchError(null);
    void apply(async () => {
      const result = await app.SyncProviderModels(deepseek.name);
      setFetchedModels(result.models);
      setFetchCount(result.models.length);
    })
      .catch((e) => {
        setFetchError(String((e as Error)?.message ?? e));
      })
      .finally(() => {
        setFetching(false);
      });
  };

  const defaultRef = (() => {
    if (!modelsReady) return "";
    const ref = toRef(s.defaultModel, s);
    return modelRefs.includes(ref) ? ref : modelRefs[0];
  })();

  const plannerRef = (() => {
    if (!modelsReady) return "";
    if (!s.plannerModel) return "";
    const ref = toRef(s.plannerModel, s);
    return modelRefs.includes(ref) ? ref : "";
  })();

  return (
    <SettingsBlock title={t("settings.providers")}>
      <div className="settings-block__form">
        <div className="set-row set-row--stack">
          <label className="set-label">{t("settings.models.baseUrlTitle")}</label>
          <DeepSeekServiceUrlField providers={s.providers} busy={busy} apply={apply} />
          <p className="settings-block__note settings-block__note--inline">{t("settings.models.baseUrlHint")}</p>
        </div>

        <div className="set-row set-row--stack">
          <label className="set-label">{t("settings.models.apiTitle")}</label>
          {deepseek?.keySet ? (
            <span className="settings-block__status">{t("settings.general.keyConfigured")}</span>
          ) : (
            <span className="settings-block__status settings-block__status--warn">{t("settings.general.keyMissing")}</span>
          )}
          {deepseek ? (
            <KeyField
              apiKeyEnv={deepseek.apiKeyEnv}
              busy={busy}
              onSet={(v) =>
                apply(() =>
                  deepseek.apiKeyEnv === "DEEPSEEK_API_KEY"
                    ? app.ConnectKey(v, deepseek.baseUrl?.trim() || "")
                    : app.SetProviderKey(deepseek.apiKeyEnv, v),
                )
              }
            />
          ) : (
            <p className="settings-block__note">{t("settings.models.apiMissingProvider")}</p>
          )}
          <p className="settings-block__note settings-block__note--inline">{t("settings.models.apiHint")}</p>
        </div>

        <div className="set-row set-row--stack">
          <label className="set-label">{t("settings.models.fetchTitle")}</label>
          <div className="settings-models-fetch-row">
            <button
              type="button"
              ref={modelsToggleRef}
              className={`settings-models-list-toggle${modelsExpanded ? " settings-models-list-toggle--open" : ""}`}
              disabled={!fetchedModels?.length}
              aria-expanded={modelsExpanded}
              onClick={() => setModelsExpanded((open) => !open)}
            >
              <ChevronDown size={13} className="settings-models-list-toggle__caret" aria-hidden="true" />
              <span>
                {fetchedModels?.length
                  ? t("settings.models.listToggle", { count: String(fetchedModels.length) })
                  : t("settings.models.listToggleEmpty")}
              </span>
            </button>
            <button
              type="button"
              className={`settings-action-btn settings-models-fetch-btn${fetching ? " settings-models-fetch-btn--loading" : ""}`}
              disabled={busy || fetching || !deepseek?.keySet}
              onClick={syncModels}
            >
              {fetching ? (
                <Loader2 size={14} className="dock-panel__spin" aria-hidden="true" />
              ) : (
                t("settings.models.fetchModels")
              )}
            </button>
          </div>
          {modelsExpanded && fetchedModels && fetchedModels.length > 0 ? (
            <div ref={modelsPanelRef} className="settings-models-list-panel" role="list">
              {fetchedModels.map((model) => (
                <span key={model} className="settings-models-list-panel__item" role="listitem">
                  {model}
                </span>
              ))}
            </div>
          ) : null}
          {fetchCount !== null ? (
            <p className="settings-block__note settings-block__note--inline settings-models-fetch-note--ok">
              {t("settings.models.fetchOk", { count: String(fetchCount) })}
            </p>
          ) : null}
          {fetchError ? <p className="settings-block__note settings-block__note--inline">{fetchError}</p> : null}
        </div>

        <div className="set-row">
          <label className="set-label">{t("settings.defaultModel")}</label>
          <StudioSelect
            className="set-grow"
            value={defaultRef}
            disabled={busy || !modelsReady}
            placeholder={selectPlaceholder}
            onChange={(value) => void apply(() => app.SetDefaultModel(value))}
            options={modelRefs.map((ref) => ({ value: ref, label: ref }))}
          />
        </div>

        <div className="set-row">
          <label className="set-label">{t("settings.plannerModel")}</label>
          <StudioSelect
            className="set-grow"
            value={plannerRef}
            disabled={busy || !modelsReady}
            placeholder={selectPlaceholder}
            onChange={(value) => void apply(() => app.SetPlannerModel(value))}
            options={[
              { value: "", label: t("settings.plannerNone") },
              ...modelRefs.map((ref) => ({ value: ref, label: ref })),
            ]}
          />
        </div>

        {s.providers.length > 1 ? (
          <div className="set-row set-row--stack">
            <label className="set-label">{t("settings.manageProviders")}</label>
            <div className="settings-block__stack">
              {s.providers.map((provider) => (
                <div key={provider.name} className="settings-provider-row">
                  <span>{provider.name}</span>
                  <button
                    type="button"
                    className="settings-provider-row__delete"
                    disabled={busy}
                    onClick={() => {
                      void (async () => {
                        const ok = await confirmAction({
                          title: t("settings.confirmDeleteProvider"),
                          message: provider.name,
                          destructive: true,
                        });
                        if (!ok) return;
                        await apply(() => app.DeleteProvider(provider.name));
                      })();
                    }}
                  >
                    {t("common.delete")}
                  </button>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </SettingsBlock>
  );
}
