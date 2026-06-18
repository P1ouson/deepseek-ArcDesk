import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { useT } from "../lib/i18n";
import { app, openExternal } from "../lib/bridge";
import { humanizeUserError } from "../lib/errors";
import { logBridgeError } from "../lib/logBridgeError";
import { normalizeProviderBaseUrl } from "./settings/modelUtils";

type ApiPreset = "deepseek" | "openrouter" | "openai" | "custom";

const PRESET_BASE: Record<Exclude<ApiPreset, "custom">, string> = {
  deepseek: "https://api.deepseek.com",
  openrouter: "https://openrouter.ai/api/v1",
  openai: "https://api.openai.com/v1",
};

const PRESET_HELP: Partial<Record<ApiPreset, string>> = {
  deepseek: "https://platform.deepseek.com/api_keys",
  openrouter: "https://openrouter.ai/keys",
  openai: "https://platform.openai.com/api-keys",
};

function detectPreset(baseUrl: string): ApiPreset {
  const base = baseUrl.trim().replace(/\/$/, "").toLowerCase();
  if (!base) return "custom";
  if (base.includes("openrouter.ai")) return "openrouter";
  if (base.includes("api.openai.com")) return "openai";
  if (base.includes("deepseek.com")) return "deepseek";
  return "custom";
}

// First-run gate: connect any OpenAI-compatible API (official, relay, OpenRouter).
export function OnboardingOverlay({
  manual = false,
  onComplete,
}: {
  manual?: boolean;
  onComplete: () => void;
}) {
  const t = useT();
  const [preset, setPreset] = useState<ApiPreset>("deepseek");
  const [baseUrl, setBaseUrl] = useState(PRESET_BASE.deepseek);
  const [value, setValue] = useState("");
  const [state, setState] = useState<"idle" | "validating" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const keyInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    void app
      .Settings()
      .then((s) => {
        const primary = s?.providers?.find((p) => p.keySet) ?? s?.providers?.find((p) => p.apiKeyEnv === "DEEPSEEK_API_KEY");
        const base = (primary?.baseUrl ?? "").replace(/\/$/, "");
        if (base) {
          setPreset(detectPreset(base));
          setBaseUrl(base);
        }
      })
      .catch((err) => logBridgeError("Onboarding.Settings", err));
  }, []);

  const applyPreset = useCallback((next: ApiPreset) => {
    setPreset(next);
    if (next !== "custom") {
      setBaseUrl(PRESET_BASE[next]);
    }
    if (state === "error") setState("idle");
  }, [state]);

  const submit = useCallback(async () => {
    const key = value.trim();
    const base = normalizeProviderBaseUrl(baseUrl.trim());
    if (!base) {
      setError(t("onboarding.error.baseUrlRequired"));
      setState("error");
      return;
    }
    if (!key) {
      setError(t("onboarding.error.empty"));
      setState("error");
      keyInputRef.current?.focus();
      return;
    }
    setState("validating");
    setError(null);
    try {
      await app.ConnectProviderAPI(base, key);
      onComplete();
    } catch (e) {
      const raw = e instanceof Error ? e.message : String(e);
      const msg = humanizeUserError(raw, t);
      if (/cancelled/i.test(raw)) {
        setError(t("onboarding.error.cancelled"));
      } else if (/status\s*401|status\s*403|unauthorized|invalid key|HTTP 401/i.test(msg)) {
        setError(t("onboarding.error.invalid"));
      } else if (/status\s*404|invalid url/i.test(raw) && /chat\/completions\/models/i.test(raw)) {
        setError(t("settings.models.relayBaseUrlError"));
      } else if (/decode response|unexpected end of json|invalid character/i.test(raw)) {
        setError(t("onboarding.error.parse", { detail: msg }));
      } else if (/Could not reach the API|无法连接 API|network|unreachable|timeout|dial tcp|connection refused|no such host/i.test(msg)) {
        setError(t("onboarding.error.network"));
      } else if (/base url is required/i.test(raw)) {
        setError(t("onboarding.error.baseUrlRequired"));
      } else if (/empty model list/i.test(raw)) {
        setError(t("onboarding.error.emptyModels"));
      } else {
        setError(msg || t("onboarding.error.unknown", { msg: raw }));
      }
      setState("error");
      keyInputRef.current?.focus();
      keyInputRef.current?.select();
    }
  }, [baseUrl, t, value, onComplete]);

  const title = manual ? t("onboarding.titleManual") : t("onboarding.title");
  const tagline = manual ? t("onboarding.taglineManual") : t("onboarding.tagline");
  const kicker = manual ? t("onboarding.kickerManual") : t("onboarding.kicker");
  const helpUrl = PRESET_HELP[preset];

  return createPortal(
    <div className="onboarding" role="dialog" aria-modal="true" aria-labelledby="onboarding-title">
      <div className="onboarding__card motion-panel-in-up">
        {manual ? (
          <button
            type="button"
            className="onboarding__close"
            aria-label={t("common.close")}
            disabled={state === "validating"}
            onClick={onComplete}
          >
            <X size={16} aria-hidden="true" />
          </button>
        ) : null}

        <header className="onboarding__head">
          <span className="onboarding__kicker">{kicker}</span>
          <h1 className="onboarding__title" id="onboarding-title">
            {title}
          </h1>
          <p className="onboarding__tag">{tagline}</p>
        </header>

        <div className="onboarding__fields">
          <div className="onboarding__field">
            <label className="onboarding__label" htmlFor="onboarding-preset">
              {t("onboarding.presetLabel")}
            </label>
            <select
              id="onboarding-preset"
              className="onboarding__input onboarding__select"
              value={preset}
              disabled={state === "validating"}
              onChange={(e) => applyPreset(e.target.value as ApiPreset)}
            >
              <option value="deepseek">{t("onboarding.preset.deepseek")}</option>
              <option value="openrouter">{t("onboarding.preset.openrouter")}</option>
              <option value="openai">{t("onboarding.preset.openai")}</option>
              <option value="custom">{t("onboarding.preset.custom")}</option>
            </select>
            <p className="onboarding__hint">{t("onboarding.presetHint")}</p>
          </div>

          <div className="onboarding__field">
            <label className="onboarding__label" htmlFor="onboarding-base-url">
              {t("onboarding.baseUrlLabel")}
            </label>
            <input
              id="onboarding-base-url"
              className="onboarding__input onboarding__input--url"
              type="url"
              autoComplete="off"
              spellCheck={false}
              placeholder={t("onboarding.baseUrlPlaceholder")}
              value={baseUrl}
              onChange={(e) => {
                setBaseUrl(e.target.value);
                setPreset(detectPreset(e.target.value));
                if (state === "error") setState("idle");
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && state !== "validating") {
                  e.preventDefault();
                  keyInputRef.current?.focus();
                }
              }}
              disabled={state === "validating"}
            />
            <p className="onboarding__hint">{t("onboarding.baseUrlHint")}</p>
          </div>

          <div className="onboarding__field">
            <label className="onboarding__label" htmlFor="onboarding-key">
              {t("onboarding.inputLabel")}
            </label>
            <input
              id="onboarding-key"
              ref={keyInputRef}
              className="onboarding__input"
              type="password"
              autoComplete="off"
              spellCheck={false}
              placeholder={t("onboarding.inputPlaceholder")}
              value={value}
              onChange={(e) => {
                setValue(e.target.value);
                if (state === "error") setState("idle");
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && state !== "validating") {
                  e.preventDefault();
                  void submit();
                }
                if (e.key === "Escape" && state !== "validating") {
                  e.preventDefault();
                  onComplete();
                }
              }}
              disabled={state === "validating"}
              autoFocus
            />
          </div>
        </div>

        {state === "error" && error ? (
          <div className="onboarding__error" role="alert">
            {error}
          </div>
        ) : null}

        <button
          type="button"
          className="onboarding__submit"
          onClick={() => void submit()}
          disabled={state === "validating"}
        >
          {state === "validating" ? (
            <>
              <span className="onboarding__spinner" aria-hidden="true" />
              {t("onboarding.validating")}
            </>
          ) : (
            t("onboarding.submit")
          )}
        </button>

        <footer className="onboarding__footer">
          <div className="onboarding__links">
            {helpUrl ? (
              <button type="button" className="onboarding__link" onClick={() => openExternal(helpUrl)}>
                {t("onboarding.getKey")}
              </button>
            ) : (
              <span className="onboarding__privacy">{t("onboarding.customKeyHint")}</span>
            )}
            {helpUrl ? (
              <>
                <span className="onboarding__sep" aria-hidden="true">
                  ·
                </span>
                <span className="onboarding__privacy">{t("onboarding.privacy")}</span>
              </>
            ) : null}
          </div>
          {!manual ? (
            <button
              type="button"
              className="onboarding__skip"
              onClick={onComplete}
              disabled={state === "validating"}
            >
              {t("onboarding.skip")}
            </button>
          ) : null}
        </footer>
      </div>
    </div>,
    document.body,
  );
}
