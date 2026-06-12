import { useCallback, useEffect, useRef, useState } from "react";
import { app } from "./bridge";
import { toErrorMessage } from "./errors";
import type { WorkspaceChangesView } from "./types";

export function useWorkspaceChanges(cwd?: string, refreshKey?: number) {
  const requestRef = useRef(0);
  const [changes, setChanges] = useState<WorkspaceChangesView | null>(null);
  const [loading, setLoading] = useState(false);

  const loadChanges = useCallback(async () => {
    const requestId = requestRef.current + 1;
    requestRef.current = requestId;
    setLoading(true);
    try {
      const next = await app.WorkspaceChanges();
      if (requestRef.current === requestId) setChanges(next);
    } catch (err) {
      if (requestRef.current === requestId) {
        setChanges({ files: [], gitAvailable: false, gitErr: toErrorMessage(err) });
      }
    } finally {
      if (requestRef.current === requestId) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadChanges();
  }, [cwd, loadChanges]);

  useEffect(() => {
    if (!refreshKey) return;
    void loadChanges();
  }, [loadChanges, refreshKey]);

  return { changes, loading, loadChanges };
}
