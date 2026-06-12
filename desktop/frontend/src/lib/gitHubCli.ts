import { toErrorMessage } from "./errors";
import type { GitPRMergeMethod } from "./types";
import type { ShellRunResult } from "./types";
import type { DictKey } from "./i18n";

export type GitHubCliProbeReason = "missing_gh" | "auth_required" | "no_pr" | "no_open_pr" | null;

export type GitHubCliProbe = {
  ghInstalled: boolean;
  ghVersion: string | null;
  ghAuthenticated: boolean;
  prNumber: number | null;
  prTitle: string | null;
  prUrl: string | null;
  prState: string | null;
  canMerge: boolean;
  reason: GitHubCliProbeReason;
};

type RunQuiet = (command: string) => Promise<ShellRunResult>;

async function runQuiet(run: RunQuiet, command: string): Promise<ShellRunResult> {
  try {
    return await run(command);
  } catch (e) {
    return { output: "", err: toErrorMessage(e) };
  }
}

export async function probeGitHubCli(run: RunQuiet): Promise<GitHubCliProbe> {
  const empty: GitHubCliProbe = {
    ghInstalled: false,
    ghVersion: null,
    ghAuthenticated: false,
    prNumber: null,
    prTitle: null,
    prUrl: null,
    prState: null,
    canMerge: false,
    reason: "missing_gh",
  };

  const version = await runQuiet(run, "gh --version");
  if (version.err && !version.output.trim()) {
    return { ...empty, reason: "missing_gh" };
  }
  const ghVersion = version.output.trim().split("\n")[0]?.trim() || null;

  const auth = await runQuiet(run, "gh auth status");
  const authText = `${auth.output}\n${auth.err ?? ""}`;
  const authOk = /logged in to github/i.test(authText);
  if (!authOk) {
    return {
      ghInstalled: true,
      ghVersion,
      ghAuthenticated: false,
      prNumber: null,
      prTitle: null,
      prUrl: null,
      prState: null,
      canMerge: false,
      reason: "auth_required",
    };
  }

  const prView = await runQuiet(run, "gh pr view --json number,state,title,url");
  if (prView.err && !prView.output.trim()) {
    return {
      ghInstalled: true,
      ghVersion,
      ghAuthenticated: true,
      prNumber: null,
      prTitle: null,
      prUrl: null,
      prState: null,
      canMerge: false,
      reason: "no_pr",
    };
  }

  try {
    const parsed = JSON.parse(prView.output.trim()) as {
      number?: number;
      state?: string;
      title?: string;
      url?: string;
    };
    const state = (parsed.state ?? "").toUpperCase();
    const open = state === "OPEN";
    return {
      ghInstalled: true,
      ghVersion,
      ghAuthenticated: true,
      prNumber: typeof parsed.number === "number" ? parsed.number : null,
      prTitle: parsed.title ?? null,
      prUrl: parsed.url ?? null,
      prState: parsed.state ?? null,
      canMerge: open && typeof parsed.number === "number",
      reason: open ? null : "no_open_pr",
    };
  } catch {
    return {
      ghInstalled: true,
      ghVersion,
      ghAuthenticated: true,
      prNumber: null,
      prTitle: null,
      prUrl: null,
      prState: null,
      canMerge: false,
      reason: "no_pr",
    };
  }
}

function mergeMethodApiFlags(method: GitPRMergeMethod): Record<string, boolean> {
  return {
    allow_merge_commit: method === "merge",
    allow_squash_merge: method === "squash",
    allow_rebase_merge: method === "rebase",
  };
}

/** Align GitHub repo allowed merge methods with the desktop preference (requires admin on repo). */
export async function syncGitHubRepoMergeMethod(
  method: GitPRMergeMethod,
  run: RunQuiet,
): Promise<{ ok: boolean; message: string }> {
  const repo = await runQuiet(run, "gh repo view --json nameWithOwner -q .nameWithOwner");
  const slug = repo.output.trim();
  if (!slug || repo.err) {
    return { ok: false, message: repo.err || "repo_unavailable" };
  }

  const flags = mergeMethodApiFlags(method);
  const args = [
    `-f allow_merge_commit=${flags.allow_merge_commit}`,
    `-f allow_squash_merge=${flags.allow_squash_merge}`,
    `-f allow_rebase_merge=${flags.allow_rebase_merge}`,
  ].join(" ");
  const result = await runQuiet(run, `gh api repos/${slug} -X PATCH ${args}`);
  if (result.err) {
    return { ok: false, message: result.err };
  }
  return { ok: true, message: slug };
}

export function probeReasonKey(reason: GitHubCliProbeReason): DictKey | null {
  switch (reason) {
    case "missing_gh":
      return "git.ghMissing";
    case "auth_required":
      return "git.ghAuthRequired";
    case "no_pr":
      return "git.ghNoPR";
    case "no_open_pr":
      return "git.ghPRNotOpen";
    default:
      return null;
  }
}
