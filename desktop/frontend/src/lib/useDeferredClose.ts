import { useCallback, useEffect, useRef, useState } from "react";
import { MOTION_DURATION_NORMAL_MS } from "./motion/constants";
import { motionDurationMs } from "./motion/prefersReducedMotion";

export type DeferredCloseStatus = "open" | "closing";

/** Defer onClose until exit animation completes (or immediately when reduced motion). */
export function useDeferredClose(onClose: () => void, durationMs = MOTION_DURATION_NORMAL_MS) {
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const [status, setStatus] = useState<DeferredCloseStatus>("open");
  const duration = motionDurationMs(durationMs);

  useEffect(() => {
    if (!onClose) return;
    setStatus("open");
  }, [onClose]);

  const requestClose = useCallback(() => {
    if (duration <= 0) {
      onCloseRef.current();
      return;
    }
    setStatus("closing");
    window.setTimeout(() => onCloseRef.current(), duration);
  }, [duration]);

  return { status, requestClose };
}
