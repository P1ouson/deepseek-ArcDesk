import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { useT } from "../lib/i18n";
import { app, openExternal } from "../lib/bridge";
import { logBridgeError } from "../lib/logBridgeError";

const DEEPSEEK_OFFICIAL_BASE = "https://api.deepseek.com";

// Full-window first-run gate: validate a pasted key via Go, then onComplete
// unmounts us so the rebuilt controller's main UI takes over.
export function OnboardingOverlay({
  manual = false,
  onComplete,
}: {
  manual?: boolean;
  onComplete: () => void;
}) {
  const t = useT();
  const [baseUrl, setBaseUrl] = useState("");
  const [value, setValue] = useState("");
  const [state, setState] = useState<"idle" | "validating" | "error">("idle");
  const [error, setError] = useState<string | null>(null);
  const keyInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    void app
      .Settings()
      .then((s) => {
        const deepseek = s?.providers?.find((p) => p.apiKeyEnv === "DEEPSEEK_API_KEY");
        const base = (deepseek?.baseUrl ?? DEEPSEEK_OFFICIAL_BASE).replace(/\/$/, "");
        if (base !== DEEPSEEK_OFFICIAL_BASE) setBaseUrl(base);
      })
      .catch((err) => logBridgeError("Onboarding.Settings", err));
  }, []);

  const submit = useCallback(async () => {
    const key = value.trim();
    if (!key) {
      setError(t("onboarding.error.empty"));
      setState("error");
      keyInputRef.current?.focus();
      return;
    }
    setState("validating");
    setError(null);
    try {
      await app.ConnectKey(key, baseUrl.trim());
      onComplete();
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      if (/status\s*401|status\s*403|invalid/i.test(msg)) {
        setError(t("onboarding.error.invalid"));
      } else if (/network|unreachable|timeout|dial/i.test(msg)) {
        setError(t("onboarding.error.network"));
      } else {
        setError(msg || t("onboarding.error.unknown"));
      }
      setState("error");
      keyInputRef.current?.focus();
      keyInputRef.current?.select();
    }
  }, [baseUrl, t, value, onComplete]);

  const title = manual ? t("onboarding.titleManual") : t("onboarding.title");
  const tagline = manual ? t("onboarding.taglineManual") : t("onboarding.tagline");
  const kicker = manual ? t("onboarding.kickerManual") : t("onboarding.kicker");

  return createPortal(
    <div className="onboarding" role="dialog" aria-modal="true" aria-labelledby="onboarding-title">
      <div className="onboarding__card motion-panel-in">
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
            <button
              type="button"
              className="onboarding__link"
              onClick={() => openExternal("https://platform.deepseek.com/api_keys")}
            >
              {t("onboarding.getKey")}
            </button>
            <span className="onboarding__sep" aria-hidden="true">
              ·
            </span>
            <span className="onboarding__privacy">{t("onboarding.privacy")}</span>
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
