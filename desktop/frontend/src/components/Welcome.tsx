import { Compass, GitBranch, SearchCode } from "lucide-react";
import logoMark from "../assets/logo.svg";
import logoWordmark from "../assets/logo-wordmark.svg";
import { useT } from "../lib/i18n";
import { ConnectionRecoveryBanner } from "./ConnectionRecoveryBanner";

type WelcomeVariant = "centered" | "code";

type QuickCard = {
  key: string;
  icon: typeof Compass;
  title: string;
  prompt: string;
};

export function Welcome({
  onPrompt,
  variant = "centered",
  disabled = false,
  showConnectionRecovery = false,
  onOpenConnectionSetup,
  workspaceName,
  workspacePath,
  showWorkspaceMesh = false,
}: {
  onPrompt: (text: string) => void | Promise<void>;
  variant?: WelcomeVariant;
  disabled?: boolean;
  showConnectionRecovery?: boolean;
  onOpenConnectionSetup?: () => void;
  workspaceName?: string;
  workspacePath?: string;
  /** Decorative gradient behind the empty workspace home (no duplicate headline). */
  showWorkspaceMesh?: boolean;
}) {
  const t = useT();

  const runPrompt = (prompt: string) => {
    if (disabled) return;
    void onPrompt(prompt);
  };

  if (variant === "code") {
    const cards: QuickCard[] = [
      {
        key: "guide",
        icon: Compass,
        title: t("welcome.code.ex1.title"),
        prompt: t("welcome.code.ex1.prompt"),
      },
      {
        key: "changes",
        icon: GitBranch,
        title: t("welcome.code.ex2.title"),
        prompt: t("welcome.code.ex2.prompt"),
      },
      {
        key: "trace",
        icon: SearchCode,
        title: t("welcome.code.ex3.title"),
        prompt: t("welcome.code.ex3.prompt"),
      },
    ];

    const boundWorkspace = workspaceName?.trim() ?? "";

    return (
      <div className={`welcome welcome--code${showWorkspaceMesh && boundWorkspace ? " welcome--code-bound" : ""}`}>
        {showWorkspaceMesh && boundWorkspace ? <div className="welcome__workspace-mesh" aria-hidden="true" /> : null}
        {showConnectionRecovery && onOpenConnectionSetup ? (
          <ConnectionRecoveryBanner onOpenSetup={onOpenConnectionSetup} />
        ) : null}
        <div className="welcome__hero">
          <img src={logoMark} className="welcome__mark" alt="" aria-hidden="true" />
          <p className="welcome__eyebrow">{t("welcome.code.eyebrow")}</p>
          {boundWorkspace && workspacePath && workspacePath !== boundWorkspace ? (
            <p className="welcome__workspace-path">{workspacePath}</p>
          ) : null}
          <h2 className="welcome__headline">
            {boundWorkspace
              ? t("welcome.code.headlineWorkspace", { name: boundWorkspace })
              : t("welcome.code.headline")}
          </h2>
        </div>

        <div className="welcome__cards-wrap">
          <p className="welcome__cards-label">{t("welcome.code.quickStart")}</p>
          <div className="welcome__cards" role="list">
            {cards.map(({ key, icon: Icon, title, prompt }) => (
              <button
                key={key}
                type="button"
                className="welcome__card"
                role="listitem"
                disabled={disabled}
                onClick={() => runPrompt(prompt)}
              >
                <span className="welcome__card-icon" aria-hidden="true">
                  <Icon size={16} strokeWidth={1.75} />
                </span>
                <span className="welcome__card-title">{title}</span>
              </button>
            ))}
          </div>
          <p className="welcome__cards-hint">{t("welcome.code.clickHint")}</p>
        </div>
      </div>
    );
  }

  const examples = [t("welcome.ex1"), t("welcome.ex2"), t("welcome.ex3")];

  return (
    <div className="welcome">
      <img src={logoWordmark} className="welcome__logo" alt="ARCDESK" />
      <div className="welcome__tag">{t("welcome.tagline")}</div>

      <div className="welcome__examples">
        {examples.map((ex) => (
          <button key={ex} type="button" className="welcome__ex" onClick={() => runPrompt(ex)}>
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
