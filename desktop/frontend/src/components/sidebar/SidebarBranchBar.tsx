import { useRef, useState, type CSSProperties } from "react";
import type { MutableRefObject } from "react";
import { Check, ChevronDown, Cloud, GitBranch, Loader2, Plus } from "lucide-react";
import { AnchoredPopover } from "../AnchoredPopover";
import { useT } from "../../lib/i18n";
import { type BranchScope, useGitBranch } from "../../lib/useGitBranch";
import { shellQuote } from "../../lib/shellQuote";

export interface SidebarBranchBarProps {
  cwd?: string;
  gitAvailable: boolean;
  refreshKey?: number;
  changeCount: number;
  addedLines: number;
  removedLines: number;
  onRefreshChanges?: () => void;
  onStageBeforeCommit?: () => Promise<void>;
}

export function SidebarBranchBar({
  cwd,
  gitAvailable,
  refreshKey = 0,
  changeCount,
  addedLines,
  removedLines,
  onRefreshChanges,
  onStageBeforeCommit,
}: SidebarBranchBarProps) {
  const t = useT();
  const {
    branch,
    localBranches,
    remoteBranches,
    defaultRemote,
    busy,
    runGit,
    checkoutBranch,
    checkoutRemoteBranch,
    createBranch,
  } = useGitBranch(cwd, gitAvailable, refreshKey);
  const branchAnchorRef = useRef<HTMLButtonElement | null>(null) as MutableRefObject<HTMLButtonElement | null>;
  const [branchScope, setBranchScope] = useState<BranchScope>("local");
  const [branchMenuOpen, setBranchMenuOpen] = useState(false);
  const [newBranch, setNewBranch] = useState("");
  const [commitOpen, setCommitOpen] = useState(false);
  const [commitMessage, setCommitMessage] = useState("");

  const pull = () => void runGit("git pull").then(() => onRefreshChanges?.());
  const push = () => void runGit("git push");

  const commitAll = () => {
    const message = commitMessage.trim();
    if (!message) return;
    void (async () => {
      await onStageBeforeCommit?.();
      const result = await runGit(`git commit -m ${shellQuote(message)}`);
      if (!result?.err) {
        setCommitMessage("");
        setCommitOpen(false);
        onRefreshChanges?.();
      }
    })();
  };

  const submitNewBranch = () => {
    const name = newBranch.trim();
    if (!name) return;
    void createBranch(name).then(() => {
      setNewBranch("");
      setBranchMenuOpen(false);
      setBranchScope("local");
      onRefreshChanges?.();
    });
  };

  const switchLocalBranch = (name: string) => {
    if (name === branch) {
      setBranchMenuOpen(false);
      return;
    }
    void checkoutBranch(name).then(() => {
      setBranchMenuOpen(false);
      onRefreshChanges?.();
    });
  };

  const switchRemoteBranch = (remoteRef: { ref: string; name: string }) => {
    if (remoteRef.name === branch) {
      setBranchMenuOpen(false);
      return;
    }
    void checkoutRemoteBranch(remoteRef).then(() => {
      setBranchMenuOpen(false);
      setBranchScope("local");
      onRefreshChanges?.();
    });
  };

  const scopeBranches = branchScope === "local" ? localBranches : remoteBranches;
  const menuHead =
    branchScope === "local"
      ? t("sidebar.switchLocalBranch")
      : t("sidebar.switchRemoteBranch", { remote: defaultRemote });

  return (
    <div className="unified-sidebar__branch">
      <div className="unified-sidebar__branch-row">
        <div
          className="unified-sidebar__scope-toggle motion-segment"
          role="group"
          aria-label={t("sidebar.branchScope")}
          style={{ "--motion-segment-count": 2, "--motion-segment-index": branchScope === "local" ? 0 : 1 } as CSSProperties}
        >
          <button
            type="button"
            className={`unified-sidebar__scope-btn${branchScope === "local" ? " unified-sidebar__scope-btn--active" : ""}`}
            disabled={!gitAvailable || busy}
            aria-pressed={branchScope === "local"}
            title={t("sidebar.localHint")}
            onClick={() => setBranchScope("local")}
          >
            <GitBranch size={13} strokeWidth={1.75} />
            <span>{t("sidebar.local")}</span>
          </button>
          <button
            type="button"
            className={`unified-sidebar__scope-btn${branchScope === "remote" ? " unified-sidebar__scope-btn--active" : ""}`}
            disabled={!gitAvailable || busy}
            aria-pressed={branchScope === "remote"}
            title={t("sidebar.remoteHint", { remote: defaultRemote })}
            onClick={() => setBranchScope("remote")}
          >
            <Cloud size={13} strokeWidth={1.75} />
            <span>{t("sidebar.remote")}</span>
          </button>
        </div>
        <button
          ref={branchAnchorRef}
          type="button"
          className="unified-sidebar__branch-name"
          disabled={!gitAvailable}
          onClick={() => setBranchMenuOpen((open) => !open)}
          aria-expanded={branchMenuOpen}
          aria-haspopup="menu"
        >
          <GitBranch size={13} strokeWidth={1.75} />
          <span>{branch || t("sidebar.noBranch")}</span>
          <ChevronDown size={12} />
        </button>
        <AnchoredPopover
          open={branchMenuOpen}
          anchorRef={branchAnchorRef}
          onClose={() => setBranchMenuOpen(false)}
          className="dock-hub-menu unified-sidebar__branch-menu"
          align="start"
          placement="bottom"
          offset={6}
        >
          <div className="unified-sidebar__branch-menu-head">{menuHead}</div>
          <div className="unified-sidebar__branch-menu-list" role="menu">
            {scopeBranches.length === 0 ? (
              <div className="unified-sidebar__branch-menu-empty">
                {branchScope === "local" ? t("sidebar.noBranches") : t("sidebar.noRemoteBranches")}
              </div>
            ) : branchScope === "local" ? (
              localBranches.map((name) => (
                <button
                  key={name}
                  type="button"
                  role="menuitem"
                  className={`dock-hub-menu__item${name === branch ? " dock-hub-menu__item--active" : ""}`}
                  onClick={() => switchLocalBranch(name)}
                  disabled={busy}
                >
                  <span className="dock-hub-menu__item-main">
                    <GitBranch size={14} />
                    <span>{name}</span>
                  </span>
                  {name === branch ? <Check size={14} /> : null}
                </button>
              ))
            ) : (
              remoteBranches.map((remoteRef) => (
                <button
                  key={remoteRef.ref}
                  type="button"
                  role="menuitem"
                  className={`dock-hub-menu__item${remoteRef.name === branch ? " dock-hub-menu__item--active" : ""}`}
                  onClick={() => switchRemoteBranch(remoteRef)}
                  disabled={busy}
                >
                  <span className="dock-hub-menu__item-main">
                    <Cloud size={14} />
                    <span>{remoteRef.name}</span>
                  </span>
                  {remoteRef.name === branch ? <Check size={14} /> : null}
                </button>
              ))
            )}
          </div>
          {branchScope === "local" ? (
            <div className="unified-sidebar__branch-menu-create">
              <input
                value={newBranch}
                onChange={(e) => setNewBranch(e.target.value)}
                placeholder={t("sidebar.newBranchPlaceholder")}
                aria-label={t("sidebar.newBranchPlaceholder")}
                onKeyDown={(e) => {
                  if (e.key === "Enter") submitNewBranch();
                }}
              />
              <button
                type="button"
                className="unified-sidebar__mini-btn"
                onClick={submitNewBranch}
                disabled={!newBranch.trim() || busy}
              >
                <Plus size={12} />
                {t("sidebar.createBranch")}
              </button>
            </div>
          ) : (
            <div className="unified-sidebar__branch-menu-foot">{t("sidebar.remoteCheckoutHint")}</div>
          )}
        </AnchoredPopover>
        <button
          type="button"
          className="unified-sidebar__commit-btn"
          disabled={!gitAvailable || busy || changeCount === 0}
          onClick={() => setCommitOpen((v) => !v)}
        >
          {busy ? <Loader2 size={13} className="spin" /> : null}
          {t("sidebar.createBranchCommit")}
          <ChevronDown size={12} />
        </button>
      </div>

      {changeCount > 0 ? (
        <div className="unified-sidebar__branch-stats">
          <span>{t("sidebar.uncommitted", { count: String(changeCount) })}</span>
          <span className="unified-sidebar__stat unified-sidebar__stat--add">+{addedLines}</span>
          <span className="unified-sidebar__stat unified-sidebar__stat--del">-{removedLines}</span>
          <span className="unified-sidebar__branch-actions">
            <button type="button" className="unified-sidebar__link" onClick={pull} disabled={busy}>
              {t("git.pull")}
            </button>
            <button type="button" className="unified-sidebar__link" onClick={push} disabled={busy}>
              {t("git.push")}
            </button>
          </span>
        </div>
      ) : null}

      {commitOpen ? (
        <div className="unified-sidebar__commit-form">
          <input
            value={commitMessage}
            onChange={(e) => setCommitMessage(e.target.value)}
            placeholder={t("git.commitLabel")}
            aria-label={t("git.commitLabel")}
            onKeyDown={(e) => {
              if (e.key === "Enter") commitAll();
            }}
          />
          <button
            type="button"
            className="unified-sidebar__mini-btn unified-sidebar__mini-btn--primary"
            onClick={commitAll}
            disabled={!commitMessage.trim() || busy}
          >
            {t("git.commit")}
          </button>
        </div>
      ) : null}
    </div>
  );
}
