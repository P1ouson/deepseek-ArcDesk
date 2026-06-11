import { useCallback, useEffect, useRef, useState } from "react";

import { ChevronDown, Loader2 } from "lucide-react";

import { app } from "../../lib/bridge";

import { useT } from "../../lib/i18n";

import { useDismissOnOutsidePointerDown } from "../../lib/useDismissOnOutsidePointerDown";

import type { ProviderView } from "../../lib/types";

import { MotionUnfold } from "../MotionUnfold";

import { StudioSelect } from "../StudioSelect";

import {

  SettingsBlock,

  SettingsSaveChip,

  type SettingsSectionProps,

} from "../settingsPrimitives";

import {

  DEEPSEEK_OFFICIAL_BASE,

  deepseekProviders,

  deepseekSyncedModels,
  formatModelFetchError,
  isRelayBaseUrl,

  looksLikeStalePresetModels,

  modelLabelFromRef,

  modelRef,

  modelShortLabel,

  normalizeProviderBaseUrl,
  primaryApiProvider,

  toRef,

} from "./modelUtils";



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

    const next = normalizeProviderBaseUrl(value.trim() || DEEPSEEK_OFFICIAL_BASE);

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



function KeyField({ busy, onSet }: { busy: boolean; onSet: (v: string) => Promise<void> }) {

  const t = useT();

  const [val, setVal] = useState("");

  return (

    <div className="set-key">

      <input

        className="mem-input"

        type="password"

        placeholder={t("settings.models.apiKeyPlaceholder")}

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

  const apiProvider = primaryApiProvider(s.providers);

  const apiModels = deepseekSyncedModels(s.providers);

  const [modelsExpanded, setModelsExpanded] = useState(false);

  const [fetchError, setFetchError] = useState<string | null>(null);

  const [fetching, setFetching] = useState(false);

  const [lastSyncedCount, setLastSyncedCount] = useState<number | null>(null);

  const autoSyncAttempted = useRef(false);

  const modelsToggleRef = useRef<HTMLButtonElement>(null);

  const modelsPanelRef = useRef<HTMLDivElement>(null);



  useDismissOnOutsidePointerDown(modelsExpanded, () => setModelsExpanded(false), {

    excludeRefs: [modelsToggleRef, modelsPanelRef],

  });



  const relayMode = isRelayBaseUrl(apiProvider?.baseUrl);



  const syncModels = useCallback(() => {

    if (!apiProvider) return Promise.resolve();

    setFetching(true);

    setFetchError(null);

    return apply(async () => {

      const result = await app.SyncProviderModels(apiProvider.name);

      setLastSyncedCount(result.models.length);

    })

      .catch((e) => {

        const raw = String((e as Error)?.message ?? e);

        setFetchError(formatModelFetchError(raw, relayMode, t));

      })

      .finally(() => {

        setFetching(false);

      });

  }, [apiProvider, apply]);



  useEffect(() => {

    if (!apiProvider?.keySet) {

      autoSyncAttempted.current = false;

      setLastSyncedCount(null);

      return;

    }

    if (apiModels.length > 0) {

      setLastSyncedCount(apiModels.length);

    }

    if (autoSyncAttempted.current || fetching || busy) return;

    const needsSync =

      apiModels.length === 0 || (relayMode && looksLikeStalePresetModels(apiModels));

    if (needsSync) {

      autoSyncAttempted.current = true;

      void syncModels();

    }

  }, [apiProvider?.keySet, apiModels, relayMode, fetching, busy, syncModels]);



  const modelRefs = apiProvider ? apiModels.map((model) => modelRef(apiProvider.name, model)) : [];

  const modelOptions = modelRefs.map((ref) => {
    const fullId = ref.includes("/") ? ref.slice(ref.indexOf("/") + 1) : ref;
    return { value: ref, label: modelLabelFromRef(ref), title: fullId };
  });

  const modelsReady = modelRefs.length > 0;

  const selectPlaceholder = t("settings.models.selectAfterFetch");

  const modelSelectMenuClass = "studio-select__menu--scroll settings-models-select-menu";



  const defaultRef = (() => {

    if (!modelsReady) return "";

    const ref = toRef(s.defaultModel, s);

    if (modelRefs.includes(ref)) return ref;

    const bare = modelLabelFromRef(ref);

    const byBare = modelRefs.find((r) => modelLabelFromRef(r) === bare);

    return byBare ?? modelRefs[0];

  })();



  const plannerRef = (() => {

    if (!modelsReady || !s.plannerModel) return "";

    const ref = toRef(s.plannerModel, s);

    if (modelRefs.includes(ref)) return ref;

    const bare = modelLabelFromRef(ref);

    return modelRefs.find((r) => modelLabelFromRef(r) === bare) ?? "";

  })();



  return (

    <SettingsBlock title={t("settings.providers")}>

      <div className="settings-block__form">

        <div className="set-row set-row--stack">

          <label className="set-label">{t("settings.models.baseUrlTitle")}</label>

          <DeepSeekServiceUrlField providers={s.providers} busy={busy} apply={apply} />

          <p className="settings-block__note settings-block__note--inline">

            {relayMode ? t("settings.models.relayHint") : t("settings.models.baseUrlHint")}

          </p>

          {relayMode ? (

            <p className="settings-block__status settings-block__status--relay">{t("settings.models.relayMode")}</p>

          ) : null}

        </div>



        <div className="set-row set-row--stack">

          <label className="set-label">{t("settings.models.apiTitle")}</label>

          {apiProvider?.keySet ? (

            <span className="settings-block__status">{t("settings.general.keyConfigured")}</span>

          ) : (

            <span className="settings-block__status settings-block__status--warn">{t("settings.general.keyMissing")}</span>

          )}

          {apiProvider ? (

            <KeyField

              busy={busy}

              onSet={(v) =>

                apply(() =>

                  apiProvider.apiKeyEnv === "DEEPSEEK_API_KEY"

                    ? app.ConnectProviderAPI(apiProvider.baseUrl?.trim() || "", v).then(() => {})

                    : app.SetProviderKey(apiProvider.apiKeyEnv, v),

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

              disabled={!apiModels.length}

              aria-expanded={modelsExpanded}

              onClick={() => setModelsExpanded((open) => !open)}

            >

              <ChevronDown size={13} className="settings-models-list-toggle__caret" aria-hidden="true" />

              <span>

                {apiModels.length

                  ? t("settings.models.listToggle", { count: String(apiModels.length) })

                  : t("settings.models.listToggleEmpty")}

              </span>

            </button>

            <button

              type="button"

              className={`settings-action-btn settings-models-fetch-btn${fetching ? " settings-models-fetch-btn--loading" : ""}`}

              disabled={busy || fetching || !apiProvider?.keySet}

              onClick={() => void syncModels()}

            >

              {fetching ? (

                <Loader2 size={14} className="dock-panel__spin" aria-hidden="true" />

              ) : (

                t("settings.models.fetchModels")

              )}

            </button>

          </div>

          {apiModels.length > 0 ? (

            <MotionUnfold open={modelsExpanded}>

              <div ref={modelsPanelRef} className="set-rules__list settings-models-list-scroll">

                <ul className="set-rules__items" role="list">

                  {apiModels.map((model) => (

                    <li key={model} className="set-rules__item" role="listitem" title={model}>

                      <span className="set-rules__item-text">{modelShortLabel(model)}</span>

                    </li>

                  ))}

                </ul>

              </div>

            </MotionUnfold>

          ) : null}

          {lastSyncedCount !== null && apiModels.length > 0 ? (

            <p className="settings-block__note settings-block__note--inline settings-models-fetch-note--ok">

              {t("settings.models.fetchOk", { count: String(apiModels.length) })}

            </p>

          ) : null}

          {fetchError ? <p className="settings-block__note settings-block__note--inline">{fetchError}</p> : null}

        </div>



        <div className="set-row">

          <label className="set-label">{t("settings.defaultModel")}</label>

          <StudioSelect

            className="set-grow settings-models-select"

            value={defaultRef}

            disabled={busy || !modelsReady}

            placeholder={selectPlaceholder}

            menuClassName={modelSelectMenuClass}

            onChange={(value) => void apply(() => app.SetDefaultModel(value))}

            options={modelOptions}

          />

        </div>



        <div className="set-row">

          <label className="set-label">{t("settings.plannerModel")}</label>

          <StudioSelect

            className="set-grow settings-models-select"

            value={plannerRef}

            disabled={busy || !modelsReady}

            placeholder={selectPlaceholder}

            menuClassName={modelSelectMenuClass}

            onChange={(value) => void apply(() => app.SetPlannerModel(value))}

            options={[{ value: "", label: t("settings.plannerNone") }, ...modelOptions]}

          />

        </div>

      </div>

    </SettingsBlock>

  );

}

