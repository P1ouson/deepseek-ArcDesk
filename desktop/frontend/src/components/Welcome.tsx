import logoWordmark from "../assets/logo-wordmark.svg";
import { useT } from "../lib/i18n";

export function Welcome({ onPrompt, variant = "centered" }: { onPrompt: (text: string) => void; variant?: "centered" | "workbench" }) {
  const t = useT();
  const examples = [t("welcome.ex1"), t("welcome.ex2"), t("welcome.ex3")];
  const workbench = variant === "workbench";

  return (
    <div className={`welcome${workbench ? " welcome--workbench" : ""}`}>
      {workbench ? (
        <>
          <h2 className="welcome__headline">{t("welcome.headline")}</h2>
          <p className="welcome__tag">{t("welcome.tagline")}</p>
        </>
      ) : (
        <>
          <img src={logoWordmark} className="welcome__logo" alt="Reasonix" />
          <div className="welcome__tag">{t("welcome.tagline")}</div>
        </>
      )}

      <div className="welcome__examples">
        {examples.map((ex) => (
          <button key={ex} type="button" className="welcome__ex" onClick={() => onPrompt(ex)}>
            {ex}
          </button>
        ))}
      </div>

      <div className="welcome__hints">
        <span>
          <kbd>/</kbd> {t("welcome.hintCommands")}
        </span>
        <span>
          <kbd>@</kbd> {t("welcome.hintFiles")}
        </span>
        <span>
          <kbd>⏎</kbd> {t("welcome.hintSend")}
        </span>
      </div>
    </div>
  );
}
