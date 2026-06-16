import { useState } from "react";
import { app } from "../../lib/bridge";
import { isNoWorkspaceRoot, isUsableCodeWorkspaceRoot } from "../../lib/composerWorkspace";
import { useT } from "../../lib/i18n";

export interface GitRepoInitBannerProps {
  cwd?: string;
  gitAvailable?: boolean;
  gitErr?: string;
  onInitialized?: () => void;
}

export function GitRepoInitBanner({
  cwd,
  gitAvailable,
  gitErr,
  onInitialized,
}: GitRepoInitBannerProps) {
  const t = useT();
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  if (gitAvailable !== false || isNoWorkspaceRoot(cwd)) {
    return null;
  }

  const handleInit = async () => {
    setBusy(true);
    setNotice(null);
    try {
      const root = cwd?.trim();
      if (!root || !isUsableCodeWorkspaceRoot(root)) {
        setNotice(t("git.initFailed"));
        return;
      }
      const result = await app.InitProjectGitRepository(root);
      if (result.err) {
        const detail = result.err.trim() || result.output.trim();
        setNotice(detail || t("git.initFailed"));
        return;
      }
      onInitialized?.();
    } finally {
      setBusy(false);
    }
  };

  const detail =
    notice ??
    (gitErr && gitErr !== "exit status 128" ? gitErr : undefined);

  return (
    <div className="dock-panel__banner dock-panel__banner--warn dock-panel__banner--actions">
      <div className="dock-panel__banner-copy">
        <p className="dock-panel__banner-text">{t("git.notARepository")}</p>
        <p className="dock-panel__banner-sub">{t("git.initRepositoryHint")}</p>
        {detail ? <p className="dock-panel__banner-sub">{detail}</p> : null}
      </div>
      <button
        type="button"
        className="dock-panel__banner-btn"
        onClick={() => void handleInit()}
        disabled={busy}
      >
        {busy ? t("git.statusRunning") : t("git.initRepository")}
      </button>
    </div>
  );
}
