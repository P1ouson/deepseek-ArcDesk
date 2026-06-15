import { useCallback, useEffect, useState } from "react";
import { app } from "./bridge";
import { shellQuote } from "./shellQuote";

export type BranchScope = "local" | "remote";

export interface RemoteBranchRef {
  ref: string;
  name: string;
}

function parseBranchList(output: string): string[] {
  return output
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

function parseRemoteBranches(output: string, remote: string): RemoteBranchRef[] {
  const prefix = `${remote}/`;
  return parseBranchList(output)
    .filter((ref) => ref.startsWith(prefix) && !ref.endsWith("/HEAD"))
    .map((ref) => ({
      ref,
      name: ref.slice(prefix.length),
    }));
}

export function useGitBranch(cwd: string | undefined, gitAvailable: boolean, refreshKey = 0) {
  const [branch, setBranch] = useState("");
  const [localBranches, setLocalBranches] = useState<string[]>([]);
  const [remoteBranches, setRemoteBranches] = useState<RemoteBranchRef[]>([]);
  const [defaultRemote, setDefaultRemote] = useState("origin");
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    if (!gitAvailable) {
      setBranch("");
      setLocalBranches([]);
      setRemoteBranches([]);
      return;
    }
    const [currentResult, localResult, remoteListResult, remoteNameResult] = await Promise.all([
      app.RunShellQuiet("git branch --show-current"),
      app.RunShellQuiet("git for-each-ref --format=%(refname:short) refs/heads/"),
      app.RunShellQuiet("git for-each-ref --format=%(refname:short) refs/remotes/"),
      app.RunShellQuiet("git remote"),
    ]);
    if (!currentResult.err) {
      setBranch(currentResult.output.trim());
    }
    if (!localResult.err) {
      setLocalBranches(parseBranchList(localResult.output));
    }
    let remote = "origin";
    if (!remoteNameResult.err) {
      const remotes = parseBranchList(remoteNameResult.output);
      if (remotes.length > 0) remote = remotes[0];
      setDefaultRemote(remote);
    }
    if (!remoteListResult.err) {
      setRemoteBranches(parseRemoteBranches(remoteListResult.output, remote));
    }
  }, [gitAvailable]);

  useEffect(() => {
    void refresh();
  }, [refresh, cwd, refreshKey]);

  const runGit = useCallback(async (command: string) => {
    setBusy(true);
    try {
      return await app.RunShellQuiet(command);
    } finally {
      setBusy(false);
    }
  }, []);

  const checkoutBranch = useCallback(
    (name: string) =>
      runGit(`git switch ${shellQuote(name)}`).then((r) => {
        void refresh();
        return r;
      }),
    [refresh, runGit],
  );

  const checkoutRemoteBranch = useCallback(
    (remoteRef: RemoteBranchRef) => {
      const run = async () => {
        if (localBranches.includes(remoteRef.name)) {
          return await runGit(`git switch ${shellQuote(remoteRef.name)}`);
        }
        const tracked = await runGit(`git switch --track ${shellQuote(remoteRef.ref)}`);
        if (!tracked.err) return tracked;
        return await runGit(`git checkout -b ${shellQuote(remoteRef.name)} ${shellQuote(remoteRef.ref)}`);
      };
      return run().then((r) => {
        void refresh();
        return r;
      });
    },
    [localBranches, refresh, runGit],
  );

  const createBranch = useCallback(
    (name: string) =>
      runGit(`git switch -c ${shellQuote(name)}`).then((r) => {
        void refresh();
        return r;
      }),
    [refresh, runGit],
  );

  return {
    branch,
    localBranches,
    remoteBranches,
    defaultRemote,
    busy,
    refresh,
    runGit,
    checkoutBranch,
    checkoutRemoteBranch,
    createBranch,
  };
}
