import { useEffect, useRef, useState } from "react";
import { toErrorMessage } from "./errors";

export type AsyncLoadState<T> = {
  data: T | undefined;
  loading: boolean;
  error: string | null;
};

/** Cancellable async effect with loading/error state — replaces copy-paste cancelled flags. */
export function useCancellableAsync<T>(
  load: (signal: { cancelled: () => boolean }) => Promise<T>,
  deps: readonly unknown[],
  initial?: T,
): AsyncLoadState<T> {
  const [data, setData] = useState<T | undefined>(initial);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const genRef = useRef(0);

  useEffect(() => {
    const gen = ++genRef.current;
    let cancelled = false;
    const isCancelled = () => cancelled || gen !== genRef.current;

    setLoading(true);
    setError(null);

    void (async () => {
      try {
        const result = await load({ cancelled: isCancelled });
        if (isCancelled()) return;
        setData(result);
      } catch (e) {
        if (isCancelled()) return;
        setError(toErrorMessage(e));
      } finally {
        if (!isCancelled()) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- caller supplies explicit deps
  }, deps);

  return { data, loading, error };
}
