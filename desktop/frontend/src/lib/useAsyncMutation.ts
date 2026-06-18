import { useCallback, useState } from "react";
import { toErrorMessage } from "./errors";

export type AsyncMutationState = {
  busy: boolean;
  err: string | null;
};

/** Shared busy/err wrapper for settings-style async saves. */
export function useAsyncMutation() {
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const run = useCallback(async (fn: () => Promise<void>) => {
    setBusy(true);
    setErr(null);
    try {
      await fn();
    } catch (e) {
      setErr(toErrorMessage(e));
    } finally {
      setBusy(false);
    }
  }, []);

  const clearErr = useCallback(() => setErr(null), []);

  return { busy, err, run, clearErr, setErr };
}
